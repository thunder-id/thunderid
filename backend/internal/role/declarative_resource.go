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

package role

import (
	"context"
	"fmt"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/group"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	resourcepkg "github.com/thunder-id/thunderid/internal/resource"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const (
	resourceTypeRole = "role"
	paramTypeRole    = "Role"
)

// roleExporter implements declarativeresource.ResourceExporter for roles.
type roleExporter struct {
	service           RoleServiceInterface
	assignmentService RoleAssignmentServiceInterface
}

// newRoleExporter creates a new role exporter.
func newRoleExporter(service RoleServiceInterface, assignmentService RoleAssignmentServiceInterface) *roleExporter {
	return &roleExporter{service: service, assignmentService: assignmentService}
}

// GetResourceType returns the resource type for roles.
func (e *roleExporter) GetResourceType() string {
	return resourceTypeRole
}

// GetParameterizerType returns the parameterizer type for roles.
func (e *roleExporter) GetParameterizerType() string {
	return paramTypeRole
}

// GetAllResourceIDs retrieves all role IDs from the database store.
// In composite mode, this excludes declarative (YAML-based) roles.
func (e *roleExporter) GetAllResourceIDs(ctx context.Context) ([]string, *tidcommon.ServiceError) {
	offset := 0
	limit := serverconst.MaxPageSize
	ids := []string{}

	for {
		roles, err := e.service.GetRoleList(ctx, limit, offset)
		if err != nil {
			return nil, err
		}

		for _, role := range roles.Roles {
			isDeclarative, svcErr := e.service.IsRoleDeclarative(ctx, role.ID)
			if svcErr != nil {
				return nil, svcErr
			}
			if !isDeclarative {
				ids = append(ids, role.ID)
			}
		}

		offset += len(roles.Roles)

		// Continue fetching while we get results; stop only on empty page
		if len(roles.Roles) == 0 {
			break
		}
	}

	return ids, nil
}

// GetResourceByID retrieves a role by its ID.
func (e *roleExporter) GetResourceByID(
	ctx context.Context, id string) (interface{}, string, *tidcommon.ServiceError) {
	roleWithPermissions, err := e.service.GetRoleWithPermissions(ctx, id)
	if err != nil {
		return nil, "", err
	}

	assignments, err := e.getAllRoleAssignments(ctx, id)
	if err != nil {
		return nil, "", err
	}

	perms := make([]roleDeclarativePermission, 0, len(roleWithPermissions.Permissions))
	for _, p := range roleWithPermissions.Permissions {
		perms = append(perms, roleDeclarativePermission(p))
	}

	role := &roleDeclarativeResource{
		ID:          roleWithPermissions.ID,
		Name:        roleWithPermissions.Name,
		Description: roleWithPermissions.Description,
		OUID:        roleWithPermissions.OUID,
		Permissions: perms,
		Assignments: assignments,
	}

	return role, role.Name, nil
}

// ValidateResource validates a role resource.
func (e *roleExporter) ValidateResource(ctx context.Context,
	resource interface{}, id string, logger *log.Logger,
) (string, *declarativeresource.ExportError) {
	role, ok := resource.(*roleDeclarativeResource)
	if !ok {
		return "", declarativeresource.CreateTypeError(resourceTypeRole, id)
	}

	if err := declarativeresource.ValidateResourceName(ctx,
		role.Name, resourceTypeRole, id, "ROLE_VALIDATION_ERROR", logger); err != nil {
		return "", err
	}

	return role.Name, nil
}

// GetResourceRules returns the parameterization rules for roles.
func (e *roleExporter) GetResourceRules() *declarativeresource.ResourceRules {
	return &declarativeresource.ResourceRules{
		Variables:      []string{},
		ArrayVariables: []string{},
	}
}

// loadDeclarativeResources loads immutable role resources from files.
// The dbStore parameter is optional (can be nil) and is used for duplicate checking in composite mode.
// The service parameter is optional (can be nil) and is used to resolve ou_handle to ou_id.
func loadDeclarativeResources(
	fileStore *fileBasedStore, dbStore roleStoreInterface, service RoleServiceInterface,
	entityService entity.EntityServiceInterface, ouService oupkg.OrganizationUnitServiceInterface,
	groupService group.GroupServiceInterface, resourceService resourcepkg.ResourceServiceInterface,
) error {
	resourceConfig := declarativeresource.ResourceConfig{
		ResourceType:  "Role",
		DirectoryName: "roles",
		Parser:        parseToRoleWrapper,
		Validator: func(data interface{}) error {
			return validateRoleWrapper(
				data, fileStore, dbStore, service, entityService, ouService, groupService, resourceService)
		},
		IDExtractor: func(data interface{}) string {
			// Use safe type assertion to prevent panic
			if v, ok := data.(*RoleWithPermissionsAndAssignments); ok {
				return v.ID
			}
			// Log error and return empty string if type assertion fails
			// Declarative resource loading runs during startup, outside any request.
			log.GetLogger().Error(context.Background(),
				"IDExtractor: type assertion failed for RoleWithPermissionsAndAssignments")
			return ""
		},
	}

	loader := declarativeresource.NewResourceLoader(resourceConfig, fileStore)
	if err := loader.LoadResources(); err != nil {
		return fmt.Errorf("failed to load role resources: %w", err)
	}

	return nil
}

// parseToRoleWrapper wraps parseToRole to match the expected signature.
func parseToRoleWrapper(data []byte) (interface{}, error) {
	return parseToRole(data)
}

type roleDeclarativePermission ResourcePermissions

type roleDeclarativeResource struct {
	ID          string                      `yaml:"id"`
	Name        string                      `yaml:"name"`
	Description string                      `yaml:"description,omitempty"`
	OUID        string                      `yaml:"ouId,omitempty"`
	OUHandle    string                      `yaml:"ouHandle,omitempty"`
	Permissions []roleDeclarativePermission `yaml:"permissions"`
	Assignments []RoleAssignment            `yaml:"assignments,omitempty"`
}

// toResourcePermissions converts roleDeclarativePermission to ResourcePermissions.
func toResourcePermissions(perm roleDeclarativePermission) ResourcePermissions {
	return ResourcePermissions(perm)
}

// parseToRole parses YAML data to RoleWithPermissionsAndAssignments.
func parseToRole(data []byte) (*RoleWithPermissionsAndAssignments, error) {
	var roleResource roleDeclarativeResource
	if err := yaml.Unmarshal(data, &roleResource); err != nil {
		return nil, err
	}

	permissions := make([]ResourcePermissions, 0, len(roleResource.Permissions))
	for _, perm := range roleResource.Permissions {
		permissions = append(permissions, toResourcePermissions(perm))
	}

	// Translate public 'user'/'app'/'agent' assignment types to the internal 'entity' type.
	for i, a := range roleResource.Assignments {
		if a.Type.IsEntityType() {
			roleResource.Assignments[i].Type = assigneeTypeEntity
		}
	}

	role := &RoleWithPermissionsAndAssignments{
		ID:          roleResource.ID,
		Name:        roleResource.Name,
		Description: roleResource.Description,
		OUID:        roleResource.OUID,
		OUHandle:    roleResource.OUHandle,
		Permissions: permissions,
		Assignments: roleResource.Assignments,
	}

	return role, nil
}

// validateRoleWrapper validates role declarative resources and checks for duplicates.
// When a service is provided, OU handles are resolved before validation runs.
func validateRoleWrapper(
	data interface{}, fileStore *fileBasedStore, dbStore roleStoreInterface, service RoleServiceInterface,
	entityService entity.EntityServiceInterface, ouService oupkg.OrganizationUnitServiceInterface,
	groupService group.GroupServiceInterface, resourceService resourcepkg.ResourceServiceInterface,
) error {
	role, ok := data.(*RoleWithPermissionsAndAssignments)
	if !ok {
		return fmt.Errorf("invalid type: expected *RoleWithPermissionsAndAssignments")
	}

	if role.ID == "" {
		return fmt.Errorf("role ID is required")
	}
	if role.Name == "" {
		return fmt.Errorf("role name is required")
	}

	if err := validateRoleOU(role, service, ouService); err != nil {
		return err
	}

	if err := validateRoleAssignments(role, entityService, groupService); err != nil {
		return err
	}

	if err := validateRolePermissions(role, resourceService); err != nil {
		return err
	}

	if fileStore != nil {
		if existingData, err := fileStore.GenericFileBasedStore.Get(role.ID); err == nil && existingData != nil {
			return fmt.Errorf("duplicate role ID '%s': role already exists in declarative resources", role.ID)
		}
	}

	if dbStore != nil {
		exists, err := dbStore.IsRoleExist(context.Background(), role.ID)
		if err != nil {
			// Fail loudly on DB errors during duplicate check
			return fmt.Errorf("checking role existence for '%s': %w", role.ID, err)
		}
		if exists {
			return fmt.Errorf("duplicate role ID '%s': role already exists in the database store", role.ID)
		}
	}

	return nil
}

// validateRoleOU resolves the role's ou_handle (when service is provided) and confirms the
// resulting ou_id refers to an existing organization unit.
func validateRoleOU(
	role *RoleWithPermissionsAndAssignments, service RoleServiceInterface,
	ouService oupkg.OrganizationUnitServiceInterface,
) error {
	if service != nil {
		if svcErr := service.ResolveRoleOUHandle(context.Background(), role); svcErr != nil {
			return fmt.Errorf("organization unit with handle %q not found for role '%s'",
				role.OUHandle, role.Name)
		}
	}
	if ouService != nil && role.OUID != "" {
		exists, svcErr := ouService.IsOrganizationUnitExists(context.Background(), role.OUID)
		if svcErr != nil {
			return fmt.Errorf("failed to check organization unit existence for role '%s': %s",
				role.Name, svcErr.Error.DefaultValue)
		}
		if !exists {
			return fmt.Errorf("organization unit with ID %q not found for role '%s'", role.OUID, role.Name)
		}
	}

	if role.OUID == "" {
		return fmt.Errorf("ouId or ouHandle is required for role '%s'", role.Name)
	}

	return nil
}

// validateRoleAssignments checks assignment shape, then, when the relevant service is available,
// confirms each assignment ID resolves to a real entity or group.
//
// This does not reuse the API path's validateAssignmentIDs: that function cross-checks each
// assignment's claimed category (user/app/agent) against the entity's actual category, but by
// the time declarative resources reach validation, parseToRole has already collapsed those public
// types into the generic internal assigneeTypeEntity, discarding the specific category. Reusing it
// here would compare "entity" against the entity's real category and always mismatch.
func validateRoleAssignments(
	role *RoleWithPermissionsAndAssignments, entityService entity.EntityServiceInterface,
	groupService group.GroupServiceInterface,
) error {
	var entityIDs, groupIDs []string
	for _, assignment := range role.Assignments {
		if assignment.ID == "" {
			return fmt.Errorf("assignment ID is required")
		}
		switch assignment.Type {
		case assigneeTypeEntity:
			entityIDs = append(entityIDs, assignment.ID)
		case AssigneeTypeGroup:
			groupIDs = append(groupIDs, assignment.ID)
		default:
			return fmt.Errorf("invalid assignment type '%s'", assignment.Type)
		}
	}
	entityIDs = utils.UniqueStrings(entityIDs)
	groupIDs = utils.UniqueStrings(groupIDs)

	if entityService != nil && len(entityIDs) > 0 {
		entities, err := entityService.GetEntitiesByIDs(context.Background(), entityIDs)
		if err != nil {
			return fmt.Errorf("failed to verify assignment entities for role '%s': %w", role.Name, err)
		}
		if len(entities) != len(entityIDs) {
			return fmt.Errorf("role '%s' references a nonexistent entity assignment", role.Name)
		}
	}

	if groupService != nil && len(groupIDs) > 0 {
		if svcErr := groupService.ValidateGroupIDs(context.Background(), groupIDs); svcErr != nil {
			return fmt.Errorf("failed to verify assignment groups for role '%s': %s",
				role.Name, svcErr.Error.DefaultValue)
		}
	}

	return nil
}

// validateRolePermissions checks that each permission entry has a resource server ID, then, when
// resourceService is available, confirms the permissions are known to that resource server.
func validateRolePermissions(
	role *RoleWithPermissionsAndAssignments, resourceService resourcepkg.ResourceServiceInterface,
) error {
	for _, resourcePerms := range role.Permissions {
		if resourcePerms.ResourceServerID == "" {
			return fmt.Errorf("resource server ID is required")
		}
		if resourceService != nil && len(resourcePerms.Permissions) > 0 {
			invalidPerms, svcErr := resourceService.ValidatePermissions(
				context.Background(), resourcePerms.ResourceServerID, resourcePerms.Permissions)
			if svcErr != nil {
				return fmt.Errorf("failed to validate permissions for role '%s': %s",
					role.Name, svcErr.Error.DefaultValue)
			}
			if len(invalidPerms) > 0 {
				return fmt.Errorf("role '%s' references unknown permissions %v for resource server '%s'",
					role.Name, invalidPerms, resourcePerms.ResourceServerID)
			}
		}
	}
	return nil
}

func (e *roleExporter) getAllRoleAssignments(
	ctx context.Context,
	roleID string,
) ([]RoleAssignment, *tidcommon.ServiceError) {
	offset := 0
	limit := serverconst.MaxPageSize
	assignments := []RoleAssignment{}

	for {
		list, err := e.assignmentService.GetRoleAssignments(ctx, roleID, limit, offset, false)
		if err != nil {
			return nil, err
		}

		for _, assignment := range list.Assignments {
			assignments = append(assignments, RoleAssignment{
				ID:   assignment.ID,
				Type: assignment.Type,
			})
		}

		offset += len(list.Assignments)

		// Continue fetching while we get results; stop only on empty page
		if len(list.Assignments) == 0 {
			break
		}
	}

	return assignments, nil
}
