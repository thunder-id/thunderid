// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"fmt"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/group"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/sharing"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const (
	resourceTypeRole = "role"
	paramTypeRole    = "Role"

	// rolePartAssignments names the assignments of a role, served under /roles/{id}/assignments.
	rolePartAssignments = "assignments"
)

// roleExporter implements declarativeresource.ResourceExporter for roles.
type roleExporter struct {
	service           RoleServiceInterface
	assignmentService RoleAssignmentServiceInterface
	sharingService    sharing.SharingServiceInterface
}

// newRoleExporter creates a new role exporter.
func newRoleExporter(
	service RoleServiceInterface, assignmentService RoleAssignmentServiceInterface,
	sharingService sharing.SharingServiceInterface,
) *roleExporter {
	return &roleExporter{service: service, assignmentService: assignmentService, sharingService: sharingService}
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
		roles, err := e.service.GetRoleList(ctx, limit, offset, nil)
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

// GetResourceByID retrieves a role by its ID, with the sharing policies recorded for it.
func (e *roleExporter) GetResourceByID(
	ctx context.Context, id string) (interface{}, string, *tidcommon.ServiceError) {
	roleWithPermissions, err := e.service.GetRoleWithPermissions(ctx, id)
	if err != nil {
		return nil, "", err
	}

	// Every assignment is written with the organization unit it was made in, so it imports back
	// there, after the sharing policies that let the role reach it.
	assignments, err := e.getAllRoleAssignments(ctx, id)
	if err != nil {
		return nil, "", err
	}

	// A document that silently lost its sharing would re-import as a role shared with nobody, so
	// this fails the export rather than thinning it.
	replayable, sharingErr := e.sharingService.ExportPolicies(ctx, RoleSharingType, id)
	if sharingErr != nil {
		return nil, "", translateSharingError(sharingErr)
	}
	var policies []sharing.PolicyRequest
	for _, r := range replayable {
		policies = append(policies, r.Request)
	}

	perms := make([]roleDeclarativePermission, 0, len(roleWithPermissions.Permissions))
	for _, p := range roleWithPermissions.Permissions {
		perms = append(perms, roleDeclarativePermission(p))
	}

	role := &roleDeclarativeResource{
		ID:              roleWithPermissions.ID,
		Name:            roleWithPermissions.Name,
		Description:     roleWithPermissions.Description,
		OUID:            roleWithPermissions.OUID,
		Permissions:     perms,
		SharingPolicies: policies,
		Assignments:     assignments,
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

// ViewResource shows an exported role as a read of it returns it.
func (e *roleExporter) ViewResource(_ context.Context, document *yaml.Node) (interface{}, error) {
	return declarativeresource.DecodeView(document, func(role *roleDeclarativeResource) (interface{}, error) {
		permissions := make([]ResourcePermissions, 0, len(role.Permissions))
		for _, perm := range role.Permissions {
			permissions = append(permissions, toResourcePermissions(perm))
		}
		return (&roleHandler{}).toHTTPRoleResponse(&RoleWithPermissions{
			ID:          role.ID,
			Name:        role.Name,
			Description: role.Description,
			OUID:        role.OUID,
			OUHandle:    role.OUHandle,
			Permissions: permissions,
		}), nil
	})
}

// ViewResourceParts shows a role's one part, "assignments", as a read of them returns them, all on
// one page.
func (e *roleExporter) ViewResourceParts(_ context.Context, document *yaml.Node) (
	map[string]interface{}, error) {
	return declarativeresource.DecodeView(document, func(role *roleDeclarativeResource) (
		map[string]interface{}, error) {
		assignments := make([]AssignmentResponse, len(role.Assignments))
		for i, a := range role.Assignments {
			assignments[i] = AssignmentResponse{ID: a.ID, Type: a.Type, OUID: a.OUID}
		}
		return map[string]interface{}{rolePartAssignments: &AssignmentListResponse{
			TotalResults: len(assignments),
			StartIndex:   1,
			Count:        len(assignments),
			Assignments:  assignments,
			Links:        []utils.Link{},
		}}, nil
	})
}

// loadDeclarativeResources loads immutable role resources from files.
// The dbStore parameter is optional (can be nil) and is used for duplicate checking in composite mode.
// The service parameter is optional (can be nil) and is used to resolve ou_handle to ou_id.
func loadDeclarativeResources(
	fileStore *fileBasedStore, dbStore roleStoreInterface, service RoleServiceInterface,
) error {
	resourceConfig := declarativeresource.ResourceConfig{
		ResourceType:  "Role",
		DirectoryName: "roles",
		Parser:        parseToRoleWrapper,
		Validator: func(data interface{}) error {
			return validateRoleWrapper(data, fileStore, dbStore, service)
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

// placeDeclaredAssignments settles the organization unit each declared assignment is made in, and
// refuses one the role cannot be assigned in. It runs once declared sharing policies have loaded,
// since an assignment outside the role's owner needs a declared policy that reaches its organization
// unit and leaves assignments editable there.
//
// An assignment naming no organization unit is made in the one owning its assignee. Applications and
// agents load after roles, so such an assignee cannot be read yet: its assignment stays in the role's
// owner, and whether it belongs in the organization unit it is assigned in is not checked (#6048).
func placeDeclaredAssignments(
	ctx context.Context, fileStore *fileBasedStore, entitySvc entity.EntityServiceInterface,
	groupSvc group.GroupServiceInterface, sharingSvc sharing.SharingServiceInterface,
) error {
	list, err := fileStore.GenericFileBasedStore.List()
	if err != nil {
		return fmt.Errorf("failed to list declared roles: %w", err)
	}
	for _, item := range list {
		role, ok := item.Data.(*RoleWithPermissionsAndAssignments)
		if !ok || role == nil || len(role.Assignments) == 0 {
			continue
		}
		if err := placeRoleAssignments(ctx, role, entitySvc, groupSvc, sharingSvc); err != nil {
			return fmt.Errorf("role '%s': %w", role.ID, err)
		}
	}
	return nil
}

// placeRoleAssignments places and checks one declared role's assignments. The role is the stored
// declaration itself, so the organization units settled here are what its reads return.
func placeRoleAssignments(
	ctx context.Context, role *RoleWithPermissionsAndAssignments, entitySvc entity.EntityServiceInterface,
	groupSvc group.GroupServiceInterface, sharingSvc sharing.SharingServiceInterface,
) error {
	var entityIDs, groupIDs []string
	for _, a := range role.Assignments {
		if a.Type == AssigneeTypeGroup {
			groupIDs = append(groupIDs, a.ID)
		} else {
			entityIDs = append(entityIDs, a.ID)
		}
	}
	entities := map[string]providers.Entity{}
	if len(entityIDs) > 0 {
		found, err := entitySvc.GetEntitiesByIDs(ctx, utils.UniqueStrings(entityIDs))
		if err != nil {
			return fmt.Errorf("failed to read the assignees: %w", err)
		}
		for _, e := range found {
			entities[e.ID] = e
		}
	}
	groups := map[string]*group.Group{}
	if len(groupIDs) > 0 {
		found, svcErr := groupSvc.GetGroupsByIDs(ctx, utils.UniqueStrings(groupIDs))
		if svcErr != nil {
			return fmt.Errorf("failed to read the group assignees: %s", svcErr.Error.DefaultValue)
		}
		groups = found
	}
	var declared map[string]sharing.Policy

	for i := range role.Assignments {
		a := &role.Assignments[i]
		e, isEntity := entities[a.ID]
		g, isGroup := groups[a.ID]
		if a.OUID == "" {
			switch {
			case a.Type == AssigneeTypeGroup && isGroup:
				a.OUID = g.OUID
			case a.Type != AssigneeTypeGroup && isEntity:
				a.OUID = e.OUID
			default:
				continue
			}
		}

		if a.OUID != role.OUID {
			overlay, svcErr := sharingSvc.ResolveOverlayRules(ctx, RoleSharingType, role.ID, a.OUID)
			if svcErr != nil && svcErr.Code != oupkg.ErrorOrganizationUnitNotFound.Code {
				return fmt.Errorf("failed to resolve the role's sharing in organization unit '%s': %s",
					a.OUID, svcErr.Error.DefaultValue)
			}
			if svcErr != nil || !overlay.Visible {
				return fmt.Errorf("assignment of '%s' is made in organization unit '%s', which the role "+
					"is not owned by or shared with", a.ID, a.OUID)
			}
			if declared == nil {
				policies, svcErr := sharingSvc.ListPolicies(ctx, RoleSharingType, role.ID)
				if svcErr != nil {
					return fmt.Errorf("failed to read the role's sharing policies: %s", svcErr.Error.DefaultValue)
				}
				declared = declaredPolicies(policies)
			}
			if !reachedThroughDeclaredPolicies(overlay.PolicyIDs, declared) {
				return fmt.Errorf("assignment of '%s' is made in organization unit '%s', which the role "+
					"reaches only through a sharing policy stored through the API, not one declared in a "+
					"resource file", a.ID, a.OUID)
			}
			if !overlay.Rules[roleAssignmentsFieldKey].Editable {
				return fmt.Errorf("assignment of '%s' is made in organization unit '%s', where the role's "+
					"sharing policies turn assignments off", a.ID, a.OUID)
			}
		}

		belongs := true
		switch {
		case a.Type == AssigneeTypeGroup && isGroup:
			belongs = g.OUID == a.OUID
		case a.Type != AssigneeTypeGroup && isEntity && e.OUID != a.OUID:
			shared, svcErr := isEntitySharedWithOU(ctx, sharingSvc, e, a.OUID)
			if svcErr != nil {
				return fmt.Errorf("failed to check whether '%s' is shared with organization unit '%s': %s",
					a.ID, a.OUID, svcErr.Error.DefaultValue)
			}
			belongs = shared
		}
		if !belongs {
			return fmt.Errorf("assignee '%s' is not owned by or shared with organization unit '%s'",
				a.ID, a.OUID)
		}
	}
	return nil
}

// declaredPolicies indexes the role's declared sharing policies by id. A stored policy is left out,
// since it can be withdrawn at runtime.
func declaredPolicies(policies []sharing.Policy) map[string]sharing.Policy {
	declared := make(map[string]sharing.Policy, len(policies))
	for _, p := range policies {
		if p.Declared {
			declared[p.ID] = p
		}
	}
	return declared
}

// reachedThroughDeclaredPolicies reports whether one of the policies reaching an organization unit
// is declared, along with every policy it was reshared beneath. Losing the role removes an
// organization unit's assignments, but not one a resource file declares, so a declared assignment is
// made only where the role's reach cannot be withdrawn at runtime.
func reachedThroughDeclaredPolicies(policyIDs []string, declared map[string]sharing.Policy) bool {
	for _, id := range policyIDs {
		p, ok := declared[id]
		for ok && p.ParentPolicyID != "" {
			p, ok = declared[p.ParentPolicyID]
		}
		if ok {
			return true
		}
	}
	return false
}

// parseToRoleWrapper wraps parseToRole to match the expected signature.
func parseToRoleWrapper(data []byte) (interface{}, error) {
	return parseToRole(data)
}

type roleDeclarativePermission ResourcePermissions

type roleDeclarativeResource struct {
	ID              string                      `yaml:"id"`
	Name            string                      `yaml:"name"`
	Description     string                      `yaml:"description,omitempty"`
	OUID            string                      `yaml:"ouId,omitempty"`
	OUHandle        string                      `yaml:"ouHandle,omitempty"`
	Permissions     []roleDeclarativePermission `yaml:"permissions"`
	SharingPolicies []sharing.PolicyRequest     `yaml:"sharingPolicies,omitempty"`
	Assignments     []RoleAssignment            `yaml:"assignments,omitempty"`
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
	if service != nil {
		if svcErr := service.ResolveRoleOUHandle(context.Background(), role); svcErr != nil {
			return fmt.Errorf("organization unit with handle %q not found for role '%s'",
				role.OUHandle, role.Name)
		}
	}
	if role.OUID == "" {
		return fmt.Errorf("ouId or ouHandle is required for role '%s'", role.Name)
	}

	for _, assignment := range role.Assignments {
		if assignment.ID == "" {
			return fmt.Errorf("assignment ID is required")
		}
		if assignment.Type != assigneeTypeEntity && assignment.Type != AssigneeTypeGroup {
			return fmt.Errorf("invalid assignment type '%s'", assignment.Type)
		}
	}

	for _, resourcePerms := range role.Permissions {
		if resourcePerms.ResourceServerID == "" {
			return fmt.Errorf("resource server ID is required")
		}
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

func (e *roleExporter) getAllRoleAssignments(
	ctx context.Context,
	roleID string,
) ([]RoleAssignment, *tidcommon.ServiceError) {
	offset := 0
	limit := serverconst.MaxPageSize
	assignments := []RoleAssignment{}

	for {
		list, err := e.assignmentService.GetRoleAssignments(ctx, roleID, "", limit, offset, false)
		if err != nil {
			return nil, err
		}

		for _, assignment := range list.Assignments {
			assignments = append(assignments, RoleAssignment{
				ID:   assignment.ID,
				Type: assignment.Type,
				OUID: assignment.OUID,
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

// makeRoleSharingConfig creates the loader config the sharing framework reads a role's declared
// policies with. It names the roles directory, because a policy is carried inside the role that
// declares it rather than in a document of its own.
func makeRoleSharingConfig(service RoleServiceInterface) sharing.DeclarativeLoaderConfig {
	return sharing.DeclarativeLoaderConfig{
		ResourceType:  RoleSharingType,
		DirectoryName: "roles",
		Parser:        makeRoleSharingParser(service),
	}
}

// makeRoleSharingParser returns a parser that reads the sharing half of one role document.
//
// It resolves through the service because a document may name its organization unit by handle,
// and a policy has to be declared against the id.
func makeRoleSharingParser(
	service RoleServiceInterface,
) func([]byte) (*sharing.DeclaredResourcePolicies, error) {
	return func(data []byte) (*sharing.DeclaredResourcePolicies, error) {
		var resource roleDeclarativeResource
		if err := yaml.Unmarshal(data, &resource); err != nil {
			return nil, fmt.Errorf("failed to parse role YAML: %w", err)
		}
		// Most documents declare no policy, and resolving one that has nothing to declare would be
		// a lookup for an answer nobody reads.
		if len(resource.SharingPolicies) == 0 {
			return &sharing.DeclaredResourcePolicies{ResourceID: resource.ID}, nil
		}

		role := &RoleWithPermissionsAndAssignments{
			ID: resource.ID, Name: resource.Name, OUID: resource.OUID, OUHandle: resource.OUHandle,
		}
		if svcErr := service.ResolveRoleOUHandle(context.Background(), role); svcErr != nil {
			return nil, fmt.Errorf("organization unit with handle %q not found for role '%s'",
				resource.OUHandle, resource.Name)
		}

		return &sharing.DeclaredResourcePolicies{
			ResourceID:   resource.ID,
			ResourceName: resource.Name,
			OwningOUID:   role.OUID,
			Policies:     resource.SharingPolicies,
		}, nil
	}
}
