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

// GetAllResourceIDs retrieves all group IDs.
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
func (e *groupExporter) ValidateResource(
	resource interface{}, id string, logger *log.Logger,
) (string, *declarativeresource.ExportError) {
	grp, ok := resource.(*groupDeclarativeResource)
	if !ok {
		return "", declarativeresource.CreateTypeError(resourceTypeGroup, id)
	}

	if err := declarativeresource.ValidateResourceName(
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
	OUID        string   `yaml:"ou_id"`
	Members     []Member `yaml:"members,omitempty"`
}
