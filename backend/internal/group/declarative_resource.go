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

package group

import (
	"context"
	"fmt"

	"bytes"

	"gopkg.in/yaml.v3"

	oupkg "github.com/thunder-id/thunderid/internal/ou"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const (
	resourceTypeGroup = "group"
	paramTypeGroup    = "Group"
)

// groupExporter implements declarativeresource.ResourceExporter for groups.
type groupExporter struct {
	service GroupServiceInterface
}

// newGroupExporter creates a new group exporter.
func newGroupExporter(service GroupServiceInterface) *groupExporter {
	return &groupExporter{service: service}
}

// GetResourceType returns the resource type for groups.
func (e *groupExporter) GetResourceType() string {
	return resourceTypeGroup
}

// GetParameterizerType returns the parameterizer type for groups.
func (e *groupExporter) GetParameterizerType() string {
	return paramTypeGroup
}

// GetAllResourceIDs retrieves all non-declarative group IDs.
// In composite mode this excludes YAML-backed groups so exports only capture mutable DB groups.
func (e *groupExporter) GetAllResourceIDs(ctx context.Context) ([]string, *serviceerror.ServiceError) {
	offset := 0
	limit := serverconst.MaxPageSize
	var ids []string

	for {
		groups, err := e.service.GetGroupList(ctx, limit, offset, false)
		if err != nil {
			return nil, err
		}

		for _, g := range groups.Groups {
			ids = append(ids, g.ID)
		}

		offset += len(groups.Groups)

		if len(groups.Groups) == 0 {
			break
		}
	}

	return ids, nil
}

// GetResourceByID retrieves a group by its ID.
func (e *groupExporter) GetResourceByID(
	ctx context.Context, id string) (interface{}, string, *serviceerror.ServiceError) {
	grp, err := e.service.GetGroup(ctx, id, false)
	if err != nil {
		return nil, "", err
	}

	members, err := e.getAllGroupMembers(ctx, id)
	if err != nil {
		return nil, "", err
	}

	exported := &groupDeclarativeResource{
		ID:          grp.ID,
		Name:        grp.Name,
		Description: grp.Description,
		OUID:        grp.OUID,
		Members:     members,
	}

	return exported, grp.Name, nil
}

// ValidateResource validates a group resource.
func (e *groupExporter) ValidateResource(ctx context.Context,
	resource interface{}, id string, logger *log.Logger,
) (string, *declarativeresource.ExportError) {
	grp, ok := resource.(*groupDeclarativeResource)
	if !ok {
		return "", declarativeresource.CreateTypeError(resourceTypeGroup, id)
	}

	if err := declarativeresource.ValidateResourceName(ctx,
		grp.Name, resourceTypeGroup, id, "GROUP_VALIDATION_ERROR", logger); err != nil {
		return "", err
	}

	return grp.Name, nil
}

// getAllGroupMembers retrieves all members of a group across all pages.
func (e *groupExporter) getAllGroupMembers(
	ctx context.Context,
	groupID string,
) ([]Member, *serviceerror.ServiceError) {
	offset := 0
	limit := serverconst.MaxPageSize
	var members []Member

	for {
		page, err := e.service.GetGroupMembers(ctx, groupID, limit, offset, false)
		if err != nil {
			return nil, err
		}

		for _, m := range page.Members {
			members = append(members, Member{
				ID:   m.ID,
				Type: m.Type,
			})
		}

		offset += len(page.Members)

		if len(page.Members) == 0 {
			break
		}
	}

	return members, nil
}

// GetResourceRules returns the parameterization rules for groups.
func (e *groupExporter) GetResourceRules() *declarativeresource.ResourceRules {
	return &declarativeresource.ResourceRules{
		Variables:      []string{},
		ArrayVariables: []string{},
	}
}

// groupDeclarativeResource represents a group as serialized in YAML for export/import.
type groupDeclarativeResource struct {
	ID          string   `yaml:"id"`
	Name        string   `yaml:"name"`
	Description string   `yaml:"description,omitempty"`
	OUID        string   `yaml:"ou_id,omitempty"`
	OUHandle    string   `yaml:"ou_handle,omitempty"`
	Members     []Member `yaml:"members,omitempty"`
}

// loadDeclarativeResources loads immutable group resources from YAML files into the file store.
// The dbStore parameter is optional and is used only for duplicate checking in composite mode.
// The ouService parameter is optional and is used to resolve ou_handle to ou_id.
func loadDeclarativeResources(
	fileStore *fileBasedGroupStore, dbStore groupStoreInterface, ouService oupkg.OrganizationUnitServiceInterface,
) error {
	resourceConfig := declarativeresource.ResourceConfig{
		ResourceType:  "Group",
		DirectoryName: "groups",
		Parser:        parseToGroupWrapper,
		Validator: func(data interface{}) error {
			return validateGroupWrapper(data, fileStore, dbStore, ouService)
		},
		IDExtractor: func(data interface{}) string {
			if v, ok := data.(*groupDeclarativeResource); ok {
				return v.ID
			}
			// Declarative resource loading runs during startup, outside any request.
			log.GetLogger().Error(context.Background(),
				"IDExtractor: type assertion failed for groupDeclarativeResource")
			return ""
		},
	}

	loader := declarativeresource.NewResourceLoader(resourceConfig, fileStore)
	if err := loader.LoadResources(); err != nil {
		return fmt.Errorf("failed to load group resources: %w", err)
	}

	return nil
}

// parseToGroupWrapper wraps parseToGroup to match the ResourceConfig.Parser signature.
func parseToGroupWrapper(data []byte) (interface{}, error) {
	return parseToGroup(data)
}

// parseToGroup parses YAML data into a groupDeclarativeResource.
// Unknown fields are rejected to surface typos in declarative config files.
func parseToGroup(data []byte) (*groupDeclarativeResource, error) {
	var grp groupDeclarativeResource
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&grp); err != nil {
		return nil, err
	}

	// Translate public 'user'/'app'/'agent' member types to the internal 'entity' type.
	for i, m := range grp.Members {
		if m.Type.IsEntityType() {
			grp.Members[i].Type = memberTypeEntity
		}
	}

	return &grp, nil
}

// validateGroupWrapper validates a parsed group and checks for duplicate IDs.
// When ouService is provided, OU handles are resolved before validation runs.
func validateGroupWrapper(
	data interface{},
	fileStore *fileBasedGroupStore,
	dbStore groupStoreInterface,
	ouService oupkg.OrganizationUnitServiceInterface,
) error {
	grp, ok := data.(*groupDeclarativeResource)
	if !ok {
		return fmt.Errorf("invalid type: expected *groupDeclarativeResource")
	}

	if grp.ID == "" {
		return fmt.Errorf("group ID is required")
	}
	if grp.Name == "" {
		return fmt.Errorf("group name is required")
	}

	if ouService != nil {
		if err := resolveGroupOUHandle(context.Background(), grp, ouService); err != nil {
			return fmt.Errorf("organization unit with handle %q not found for group '%s': %w",
				grp.OUHandle, grp.Name, err)
		}
	}

	if grp.OUID == "" {
		return fmt.Errorf("ou_id or ou_handle is required for group '%s'", grp.Name)
	}

	if fileStore != nil {
		if existing, err := fileStore.GenericFileBasedStore.Get(grp.ID); err == nil && existing != nil {
			return fmt.Errorf("duplicate group ID '%s': group already exists in declarative resources", grp.ID)
		}
	}

	if dbStore != nil {
		grpDAO, err := dbStore.GetGroup(context.Background(), grp.ID)
		if err == nil && grpDAO.ID != "" {
			return fmt.Errorf("duplicate group ID '%s': group already exists in the database store", grp.ID)
		} else if err != nil && !isGroupNotFoundError(err) {
			return fmt.Errorf("failed to check for duplicate group ID '%s': %w", grp.ID, err)
		}
	}

	return nil
}
