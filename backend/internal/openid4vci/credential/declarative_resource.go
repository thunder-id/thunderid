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

package credential

import (
	"context"
	"fmt"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"

	"gopkg.in/yaml.v3"
)

const (
	resourceTypeCredentialConfiguration = "credential_configuration" //nolint:gosec
	paramTypeCredentialConfiguration    = "CredentialConfiguration"  //nolint:gosec
)

// configurationExporter implements declarativeresource.ResourceExporter for
// OpenID4VCI credential configurations, reading them through the service.
type configurationExporter struct {
	service CredentialConfigurationServiceInterface
}

// newConfigurationExporter creates a configurationExporter backed by the given credential configuration service.
func newConfigurationExporter(service CredentialConfigurationServiceInterface) *configurationExporter {
	return &configurationExporter{service: service}
}

// GetResourceType returns the resource type identifier for credential configurations.
func (e *configurationExporter) GetResourceType() string {
	return resourceTypeCredentialConfiguration
}

// GetParameterizerType returns the parameterizer type name for credential configurations.
func (e *configurationExporter) GetParameterizerType() string {
	return paramTypeCredentialConfiguration
}

// GetAllResourceIDs returns the IDs of all mutable (database-backed) credential
// configurations, excluding any declarative (file-based) configurations.
func (e *configurationExporter) GetAllResourceIDs(ctx context.Context) ([]string, *tidcommon.ServiceError) {
	configs, err := e.service.ListCredentialConfigurations(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(configs))
	for _, dto := range configs {
		isDeclarative, svcErr := e.service.IsCredentialConfigurationDeclarative(ctx, dto.ID)
		if svcErr != nil {
			return nil, svcErr
		}
		if !isDeclarative {
			ids = append(ids, dto.ID)
		}
	}
	return ids, nil
}

// GetResourceByID retrieves a credential configuration by ID for export, returning
// its handle as the stable resource name.
func (e *configurationExporter) GetResourceByID(ctx context.Context, id string) (
	interface{}, string, *tidcommon.ServiceError,
) {
	dto, err := e.service.GetCredentialConfiguration(ctx, id)
	if err != nil {
		return nil, "", err
	}
	dto.OUHandle = ""
	return dto, dto.Handle, nil
}

// ValidateResource validates a credential configuration prior to export,
// extracting its handle as the stable resource name.
func (e *configurationExporter) ValidateResource(ctx context.Context,
	resource interface{}, id string, logger *log.Logger) (string, *declarativeresource.ExportError) {
	dto, ok := resource.(*CredentialConfigurationDTO)
	if !ok {
		return "", declarativeresource.CreateTypeError(resourceTypeCredentialConfiguration, id)
	}
	if err := declarativeresource.ValidateResourceName(ctx,
		dto.Handle, resourceTypeCredentialConfiguration, id, "VCI_CONFIGURATION_VALIDATION_ERROR", logger); err != nil {
		return "", err
	}
	return dto.Handle, nil
}

// GetResourceRules returns the parameterization rules for credential configurations.
func (e *configurationExporter) GetResourceRules() *declarativeresource.ResourceRules {
	return &declarativeresource.ResourceRules{}
}

// configurationRequestWithID is the YAML shape of a declarative credential
// configuration: the management request body plus the stable resource ID.
type configurationRequestWithID struct {
	ID              string             `yaml:"id"`
	Handle          string             `yaml:"handle"`
	Format          string             `yaml:"format"`
	VCT             string             `yaml:"vct"`
	Claims          []ClaimMapping     `yaml:"claims"`
	Display         *CredentialDisplay `yaml:"display"`
	ValiditySeconds *int               `yaml:"validitySeconds"`
}

// loadDeclarativeResources loads declarative credential-configuration resources from files.
func loadDeclarativeResources(store declarativeresource.Storer) error {
	resourceConfig := declarativeresource.ResourceConfig{
		ResourceType:  paramTypeCredentialConfiguration,
		DirectoryName: "credential_configurations",
		Parser:        parseToConfigurationDTOWrapper,
		Validator:     validateConfigurationWrapper,
		IDExtractor: func(dto interface{}) string {
			return dto.(*CredentialConfigurationDTO).ID
		},
	}
	loader := declarativeresource.NewResourceLoader(resourceConfig, store)
	if err := loader.LoadResources(); err != nil {
		return fmt.Errorf("failed to load credential configuration resources: %w", err)
	}
	return nil
}

// parseToConfigurationDTOWrapper unmarshals YAML declarative resource data into a CredentialConfigurationDTO.
func parseToConfigurationDTOWrapper(data []byte) (interface{}, error) {
	var req configurationRequestWithID
	if err := yaml.Unmarshal(data, &req); err != nil {
		return nil, err
	}
	return &CredentialConfigurationDTO{
		ID:              req.ID,
		Handle:          req.Handle,
		Format:          req.Format,
		VCT:             req.VCT,
		Claims:          req.Claims,
		Display:         req.Display,
		ValiditySeconds: req.ValiditySeconds,
	}, nil
}

// validateConfigurationWrapper validates a declarative credential configuration DTO, requiring an ID and valid fields.
func validateConfigurationWrapper(dto interface{}) error {
	cfg, ok := dto.(*CredentialConfigurationDTO)
	if !ok {
		return fmt.Errorf("invalid type: expected *CredentialConfigurationDTO")
	}
	if cfg.ID == "" {
		return fmt.Errorf("credential configuration ID is required")
	}
	if svcErr := validateConfiguration(cfg); svcErr != nil {
		return fmt.Errorf("validation failed: %s", svcErr.Error.DefaultValue)
	}
	return nil
}
