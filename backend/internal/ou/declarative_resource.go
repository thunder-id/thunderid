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

package ou

import (
	"context"
	"fmt"
	"testing"

	"gopkg.in/yaml.v3"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const (
	resourceTypeOU = "organization_unit"
	paramTypeOU    = "OrganizationUnit"
)

// ouExporter implements declarativeresource.ResourceExporter for organization units.
type ouExporter struct {
	service OrganizationUnitServiceInterface
}

// newOUExporter creates a new OU exporter.
func newOUExporter(service OrganizationUnitServiceInterface) *ouExporter {
	return &ouExporter{service: service}
}

// NewOUExporterForTest creates a new OU exporter for testing purposes.
func NewOUExporterForTest(service OrganizationUnitServiceInterface) *ouExporter {
	if !testing.Testing() {
		panic("only for tests!")
	}
	return newOUExporter(service)
}

// GetResourceType returns the resource type for organization units.
func (e *ouExporter) GetResourceType() string {
	return resourceTypeOU
}

// GetParameterizerType returns the parameterizer type for organization units.
func (e *ouExporter) GetParameterizerType() string {
	return paramTypeOU
}

// GetAllResourceIDs retrieves all organization unit IDs from the database store.
// Note: This only exports DB-backed OUs (runtime OUs). YAML-based declarative resources
// are not included in the export as they are already defined in YAML files.
func (e *ouExporter) GetAllResourceIDs(ctx context.Context) ([]string, *serviceerror.ServiceError) {
	// Get all OUs by requesting a large limit from the service
	// In composite mode, this returns OUs from both file-based and database stores
	ous, err := e.service.GetOrganizationUnitList(ctx, serverconst.MaxPageSize, 0, nil)
	if err != nil {
		return nil, err
	}

	// Collect only mutable OUs (exclude immutable OUs from file store)
	// In composite mode, we need to filter out declarative resources
	ids := make([]string, 0, len(ous.OrganizationUnits))
	for _, ouBasic := range ous.OrganizationUnits {
		// Only include mutable OUs (exclude immutable ones)
		if !e.service.IsOrganizationUnitDeclarative(ctx, ouBasic.ID) {
			ids = append(ids, ouBasic.ID)
		}
	}

	// Also get all child OUs recursively (only mutable ones)
	allIDs := make(map[string]bool)
	for _, id := range ids {
		allIDs[id] = true
		childIDs, err := e.getAllChildIDs(ctx, id)
		if err != nil {
			return nil, err
		}
		for _, childID := range childIDs {
			allIDs[childID] = true
		}
	}

	result := make([]string, 0, len(allIDs))
	for id := range allIDs {
		result = append(result, id)
	}

	return result, nil
}

// getAllChildIDs recursively retrieves all child OU IDs (excluding immutable ones).
func (e *ouExporter) getAllChildIDs(ctx context.Context, parentID string) ([]string, *serviceerror.ServiceError) {
	children, err := e.service.GetOrganizationUnitChildren(ctx, parentID, serverconst.MaxPageSize, 0, nil)
	if err != nil {
		return nil, err
	}

	allIDs := []string{}
	for _, childBasic := range children.OrganizationUnits {
		// Only include mutable children (exclude immutable ones)
		if !e.service.IsOrganizationUnitDeclarative(ctx, childBasic.ID) {
			allIDs = append(allIDs, childBasic.ID)
			grandchildIDs, err := e.getAllChildIDs(ctx, childBasic.ID)
			if err != nil {
				return nil, err
			}
			allIDs = append(allIDs, grandchildIDs...)
		}
	}

	return allIDs, nil
}

// GetResourceByID retrieves an organization unit by its ID.
func (e *ouExporter) GetResourceByID(
	ctx context.Context, id string) (interface{}, string, *serviceerror.ServiceError) {
	ou, err := e.service.GetOrganizationUnit(ctx, id)
	if err != nil {
		return nil, "", err
	}
	return &ou, ou.Name, nil
}

// ValidateResource validates an organization unit resource.
func (e *ouExporter) ValidateResource(
	resource interface{}, id string, logger *log.Logger,
) (string, *declarativeresource.ExportError) {
	ou, ok := resource.(*OrganizationUnit)
	if !ok {
		return "", declarativeresource.CreateTypeError(resourceTypeOU, id)
	}

	if err := declarativeresource.ValidateResourceName(
		ou.Name, resourceTypeOU, id, "OU_VALIDATION_ERROR", logger); err != nil {
		return "", err
	}

	return ou.Name, nil
}

// GetResourceRules returns the parameterization rules for organization units.
func (e *ouExporter) GetResourceRules() *declarativeresource.ResourceRules {
	// OUs typically don't have parameterizable fields
	return &declarativeresource.ResourceRules{
		Variables:      []string{},
		ArrayVariables: []string{},
	}
}

// loadDeclarativeResources loads immutable organization unit resources from files.
// The dbStore parameter is optional (can be nil) and is used for duplicate checking in composite mode.
func loadDeclarativeResources(fileStore organizationUnitStoreInterface, dbStore organizationUnitStoreInterface) error {
	// Type assert to get the file-based store for resource loading
	store, ok := fileStore.(*fileBasedStore)
	if !ok {
		return fmt.Errorf("fileStore must be a file-based store implementation")
	}

	resourceConfig := declarativeresource.ResourceConfig{
		ResourceType:  "OrganizationUnit",
		DirectoryName: "organization_units",
		Parser:        parseToOUWrapper,
		Validator: func(data interface{}) error {
			return validateOUWrapper(data, store, dbStore)
		},
		IDExtractor: func(data interface{}) string {
			return data.(*OrganizationUnit).ID
		},
	}

	loader := declarativeresource.NewResourceLoader(resourceConfig, store)
	if err := loader.LoadResources(); err != nil {
		return fmt.Errorf("failed to load organization unit resources: %w", err)
	}

	return nil
}

// parseToOUWrapper wraps parseToOU to match the expected signature.
func parseToOUWrapper(data []byte) (interface{}, error) {
	return parseToOU(data)
}

// parseToOU parses YAML data to OrganizationUnit.
func parseToOU(data []byte) (*OrganizationUnit, error) {
	var ou OrganizationUnit
	err := yaml.Unmarshal(data, &ou)
	if err != nil {
		return nil, err
	}

	return &ou, nil
}

// validateOUWrapper wraps validateOU to match ResourceConfig.Validator signature.
// Checks for duplicate IDs in both the file store and optionally the database store.
// In declarative mode, dbStore is nil and only file store is checked.
// In composite mode, both stores are checked to prevent conflicts.
func validateOUWrapper(data interface{}, fileStore *fileBasedStore, dbStore organizationUnitStoreInterface) error {
	ou, ok := data.(*OrganizationUnit)
	if !ok {
		return fmt.Errorf("invalid type: expected *OrganizationUnit")
	}

	if ou.ID == "" {
		return fmt.Errorf("organization unit ID is required")
	}

	if ou.Name == "" {
		return fmt.Errorf("organization unit name is required")
	}

	if ou.Handle == "" {
		return fmt.Errorf("organization unit handle is required")
	}

	// Check for duplicate ID in the file store
	if existingData, err := fileStore.GenericFileBasedStore.Get(ou.ID); err == nil && existingData != nil {
		return fmt.Errorf("duplicate organization unit ID '%s': "+
			"an organization unit with this ID already exists in declarative resources", ou.ID)
	}

	// Check for duplicate ID in the database store (only in composite mode)
	if dbStore != nil {
		exists, err := dbStore.IsOrganizationUnitExists(context.Background(), ou.ID)
		if err != nil {
			return fmt.Errorf("failed to check organization unit existence for ID '%s': %w", ou.ID, err)
		}
		if exists {
			return fmt.Errorf("duplicate organization unit ID '%s': "+
				"an organization unit with this ID already exists in the database store", ou.ID)
		}
	}

	return nil
}
