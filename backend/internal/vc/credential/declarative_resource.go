// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package credential

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
	OUID            string             `yaml:"ouId"`
	OUHandle        string             `yaml:"ouHandle"`
	Name            string             `yaml:"name"`
	Description     string             `yaml:"description"`
	Format          string             `yaml:"format"`
	VCT             string             `yaml:"vct"`
	Claims          []ClaimMapping     `yaml:"claims"`
	Display         *CredentialDisplay `yaml:"display"`
	ValiditySeconds *int               `yaml:"validitySeconds"`
}

// loadDeclarativeResources loads declarative credential-configuration resources from YAML files
// into the file store. The dbStore parameter is optional and is used only for duplicate checking
// in composite mode. The ouService parameter is optional and is used to resolve ouHandle to ouId.
func loadDeclarativeResources(
	fileStore *credentialFileBasedStore, dbStore credentialStoreInterface,
	ouService ou.OrganizationUnitServiceInterface,
) error {
	resourceConfig := declarativeresource.ResourceConfig{
		ResourceType:  paramTypeCredentialConfiguration,
		DirectoryName: "credential_configurations",
		Parser:        parseToConfigurationDTOWrapper,
		Validator: func(dto interface{}) error {
			return validateConfigurationWrapper(dto, fileStore, dbStore, ouService)
		},
		IDExtractor: func(dto interface{}) string {
			return dto.(*CredentialConfigurationDTO).ID
		},
	}
	loader := declarativeresource.NewResourceLoader(resourceConfig, &credentialStorer{store: fileStore})
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
		OUID:            req.OUID,
		OUHandle:        req.OUHandle,
		Name:            req.Name,
		Description:     req.Description,
		Format:          req.Format,
		VCT:             req.VCT,
		Claims:          req.Claims,
		Display:         req.Display,
		ValiditySeconds: req.ValiditySeconds,
	}, nil
}

// validateConfigurationWrapper validates a declarative credential configuration: it must carry
// an ID, pass the same field validation the management API applies, resolve to an existing
// organization unit, and not reuse an ID or handle already claimed by another file.
func validateConfigurationWrapper(
	dto interface{}, fileStore *credentialFileBasedStore, dbStore credentialStoreInterface,
	ouService ou.OrganizationUnitServiceInterface,
) error {
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
	if err := resolveConfigurationOU(context.Background(), cfg, ouService); err != nil {
		return err
	}
	return checkDuplicateConfiguration(context.Background(), cfg, fileStore, dbStore)
}

// resolveConfigurationOU resolves ouHandle to ouId and requires the result to be non-empty, so
// a declarative configuration carries the same owning organization unit the management API demands.
func resolveConfigurationOU(
	ctx context.Context, cfg *CredentialConfigurationDTO, ouService ou.OrganizationUnitServiceInterface,
) error {
	if ouService != nil && cfg.OUID == "" && cfg.OUHandle != "" {
		resolved, svcErr := ouService.GetOrganizationUnitByPath(ctx, cfg.OUHandle)
		if svcErr != nil {
			return fmt.Errorf("organization unit with handle %q not found for credential configuration '%s'",
				cfg.OUHandle, cfg.Handle)
		}
		cfg.OUID = resolved.ID
	}
	if cfg.OUID == "" {
		return fmt.Errorf("ouId or ouHandle is required for credential configuration '%s'", cfg.Handle)
	}
	return nil
}

// checkDuplicateConfiguration rejects a configuration whose ID or handle another declarative file
// already claimed, which would otherwise silently overwrite or shadow the earlier one.
func checkDuplicateConfiguration(
	ctx context.Context, cfg *CredentialConfigurationDTO,
	fileStore *credentialFileBasedStore, dbStore credentialStoreInterface,
) error {
	if fileStore != nil {
		if _, err := fileStore.GetCredentialConfigurationByID(ctx, cfg.ID); err == nil {
			return fmt.Errorf(
				"duplicate credential configuration ID '%s': configuration already exists in declarative resources",
				cfg.ID)
		}
		if _, err := fileStore.GetCredentialConfigurationByHandle(ctx, cfg.Handle); err == nil {
			return fmt.Errorf(
				"duplicate credential configuration handle '%s': handle already used in declarative resources",
				cfg.Handle)
		}
	}
	if dbStore != nil {
		_, err := dbStore.GetCredentialConfigurationByID(ctx, cfg.ID)
		if err == nil {
			return fmt.Errorf(
				"duplicate credential configuration ID '%s': configuration already exists in the database store",
				cfg.ID)
		} else if !errors.Is(err, ErrNotFound) {
			return fmt.Errorf("failed to check for duplicate credential configuration ID '%s': %w", cfg.ID, err)
		}
	}
	return nil
}
