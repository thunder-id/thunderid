// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/agent"
	agentmodel "github.com/thunder-id/thunderid/internal/agent/model"
	authzdetailmgt "github.com/thunder-id/thunderid/internal/authzdetail/mgt"
	layoutmgt "github.com/thunder-id/thunderid/internal/design/layout/mgt"
	thememgt "github.com/thunder-id/thunderid/internal/design/theme/mgt"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/notificationtemplate"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/sharing"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/user"
	"github.com/thunder-id/thunderid/internal/vc/credential"
	"github.com/thunder-id/thunderid/internal/vc/presentation"
)

// resolveImportOUHandle resolves an ou_handle to its corresponding OU ID for import operations.
// If both ouID and ouHandle are provided, ouID wins and a warning is logged.
// Returns the (possibly resolved) ouID and any service error from the OU lookup.
func (s *importService) resolveImportOUHandle(
	ctx context.Context, resourceType, resourceID, resourceName, ouID, ouHandle string,
) (string, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ImportService"))
	if ouID != "" && ouHandle != "" {
		logger.Warn(ctx, "Both ouId and ouHandle provided; ouHandle ignored",
			log.String("resourceType", resourceType),
			log.String("resourceID", resourceID),
			log.String("resourceName", resourceName))
		return ouID, nil
	}
	if ouID == "" && ouHandle != "" {
		if s.ouService == nil {
			return "", &tidcommon.InternalServerError
		}
		resolved, svcErr := s.ouService.GetOrganizationUnitByPath(ctx, ouHandle)
		if svcErr != nil {
			return "", svcErr
		}
		return resolved.ID, nil
	}
	return ouID, nil
}

// documentState returns a document's top-level state and whether it has a non-empty one.
func documentState(doc parsedDocument) (string, bool) {
	for i := 0; i+1 < len(doc.Node.Content); i += 2 {
		if doc.Node.Content[i].Value != "state" {
			continue
		}
		value := doc.Node.Content[i+1]
		return value.Value, value.Value != "" || len(value.Content) > 0
	}
	return "", false
}

type roleDeclarativeYAML struct {
	ID              string                     `yaml:"id"`
	Name            string                     `yaml:"name"`
	Description     string                     `yaml:"description,omitempty"`
	OUID            string                     `yaml:"ouId,omitempty"`
	OUHandle        string                     `yaml:"ouHandle,omitempty"`
	Permissions     []role.ResourcePermissions `yaml:"permissions"`
	SharingPolicies []sharing.PolicyRequest    `yaml:"sharingPolicies,omitempty"`
	Assignments     []role.RoleAssignment      `yaml:"assignments,omitempty"`
}

type userDeclarativeYAML struct {
	ID              string                  `yaml:"id"`
	Type            string                  `yaml:"type"`
	OUID            string                  `yaml:"ouId,omitempty"`
	OUHandle        string                  `yaml:"ouHandle,omitempty"`
	Attributes      map[string]interface{}  `yaml:"attributes"`
	Credentials     map[string]interface{}  `yaml:"credentials,omitempty"`
	SharingPolicies []sharing.PolicyRequest `yaml:"sharingPolicies,omitempty"`
}

type entityTypeDeclarativeYAML struct {
	ID                    string                       `yaml:"id"`
	Category              entitytype.TypeCategory      `yaml:"category,omitempty"`
	Handle                string                       `yaml:"handle"`
	DisplayName           string                       `yaml:"displayName"`
	OUID                  string                       `yaml:"ouId,omitempty"`
	OUHandle              string                       `yaml:"ouHandle,omitempty"`
	AllowSelfRegistration bool                         `yaml:"allowSelfRegistration,omitempty"`
	SystemAttributes      *entitytype.SystemAttributes `yaml:"systemAttributes,omitempty"`
	Schema                interface{}                  `yaml:"schema"`
}

type themeDeclarativeYAML struct {
	ID          string      `yaml:"id"`
	Handle      string      `yaml:"handle"`
	DisplayName string      `yaml:"displayName"`
	Description string      `yaml:"description,omitempty"`
	Theme       interface{} `yaml:"theme"`
}

type layoutDeclarativeYAML struct {
	ID          string      `yaml:"id"`
	Handle      string      `yaml:"handle"`
	DisplayName string      `yaml:"displayName"`
	Description string      `yaml:"description,omitempty"`
	Layout      interface{} `yaml:"layout"`
}

// notificationTemplateDeclarativeYAML is the import shape for a notification template.
// It mirrors the API create model with an added channel discriminator.
type notificationTemplateDeclarativeYAML struct {
	ID          string `yaml:"id,omitempty"`
	Channel     string `yaml:"channel"`
	Handle      string `yaml:"handle"`
	OUID        string `yaml:"ouId,omitempty"`
	OUHandle    string `yaml:"ouHandle,omitempty"`
	DisplayName string `yaml:"displayName"`
	Description string `yaml:"description"`
	Content     struct {
		Subject string `yaml:"subject"`
		Body    string `yaml:"body"`
	} `yaml:"content"`
	Design *struct {
		ColorScheme string `yaml:"colorScheme"`
	} `yaml:"design"`
}

func (s *importService) importOrganizationUnit(
	ctx context.Context, doc parsedDocument, options *ImportOptions, dryRun bool,
) ImportItemOutcome {
	if s.ouService == nil {
		return unsupportedAdapterOutcome(resourceTypeOrganizationUnit, "organization unit")
	}

	var req ou.OrganizationUnit
	if err := doc.Node.Decode(&req); err != nil {
		return decodeErrorOutcome(resourceTypeOrganizationUnit, req.ID, req.Name, err)
	}

	createReq := ou.OrganizationUnitRequestWithID{
		ID:                        req.ID,
		Handle:                    req.Handle,
		Name:                      req.Name,
		Description:               req.Description,
		Parent:                    req.Parent,
		ThemeID:                   req.ThemeID,
		LayoutID:                  req.LayoutID,
		AuthFlowID:                req.AuthFlowID,
		RegistrationFlowID:        req.RegistrationFlowID,
		IsRegistrationFlowEnabled: req.IsRegistrationFlowEnabled,
		RecoveryFlowID:            req.RecoveryFlowID,
		IsRecoveryFlowEnabled:     req.IsRecoveryFlowEnabled,
		SignOutFlowID:             req.SignOutFlowID,
		LogoURL:                   req.LogoURL,
		TosURI:                    req.TosURI,
		PolicyURI:                 req.PolicyURI,
		CookiePolicyURI:           req.CookiePolicyURI,
	}
	updateReq := createReq

	if dryRun {
		if options.IsUpsertEnabled() && req.ID != "" {
			_, svcErr := s.ouService.GetOrganizationUnit(ctx, req.ID)
			if svcErr == nil {
				return successOutcome(resourceTypeOrganizationUnit, req.ID, req.Name, operationUpdate)
			}

			if !isNotFoundServiceError(svcErr) {
				return serviceErrorOutcome(resourceTypeOrganizationUnit, req.ID, req.Name, operationUpdate, svcErr)
			}
		}

		return successOutcome(resourceTypeOrganizationUnit, req.ID, req.Name, operationCreate)
	}

	if options.IsUpsertEnabled() && req.ID != "" {
		updated, svcErr := s.ouService.UpdateOrganizationUnit(ctx, req.ID, updateReq)
		if svcErr == nil {
			return successOutcome(resourceTypeOrganizationUnit, updated.ID, updated.Name, operationUpdate)
		}

		if !isNotFoundServiceError(svcErr) {
			return serviceErrorOutcome(resourceTypeOrganizationUnit, req.ID, req.Name, operationUpdate, svcErr)
		}

		created, createErr := s.ouService.CreateOrganizationUnit(ctx, createReq)
		if createErr != nil {
			return serviceErrorOutcome(resourceTypeOrganizationUnit, req.ID, req.Name, operationCreate, createErr)
		}

		return successOutcome(resourceTypeOrganizationUnit, created.ID, created.Name, operationCreate)
	}

	created, svcErr := s.ouService.CreateOrganizationUnit(ctx, createReq)
	if svcErr != nil {
		return serviceErrorOutcome(resourceTypeOrganizationUnit, req.ID, req.Name, operationCreate, svcErr)
	}

	return successOutcome(resourceTypeOrganizationUnit, created.ID, created.Name, operationCreate)
}

func (s *importService) importEntityType(
	ctx context.Context, doc parsedDocument, options *ImportOptions, dryRun bool,
) ImportItemOutcome {
	if s.entityTypeService == nil {
		return unsupportedAdapterOutcome(resourceTypeEntityType, "user type")
	}

	var req entityTypeDeclarativeYAML
	if err := doc.Node.Decode(&req); err != nil {
		return decodeErrorOutcome(resourceTypeEntityType, req.ID, req.DisplayName, err)
	}

	var (
		schemaBytes []byte
		err         error
	)
	switch v := req.Schema.(type) {
	case string:
		schemaBytes = []byte(v)
	default:
		schemaBytes, err = json.Marshal(v)
		if err != nil {
			return ImportItemOutcome{
				ResourceType: resourceTypeEntityType,
				ResourceID:   req.ID,
				ResourceName: req.DisplayName,
				Status:       statusFailed,
				Code:         ErrorInvalidYAMLContent.Code,
				Message:      fmt.Sprintf("failed to marshal schema: %v", err),
			}
		}
	}

	category := req.Category
	if category == "" {
		if doc.ResourceType == resourceTypeAgentType {
			category = entitytype.TypeCategoryAgent
		} else {
			category = entitytype.TypeCategoryUser
		}
	}
	if !category.IsValid() {
		return ImportItemOutcome{
			ResourceType: resourceTypeEntityType,
			ResourceID:   req.ID,
			ResourceName: req.DisplayName,
			Status:       statusFailed,
			Code:         ErrorInvalidYAMLContent.Code,
			Message:      fmt.Sprintf("invalid entity type category %q", string(category)),
		}
	}

	createReq := entitytype.CreateEntityTypeRequestWithID{
		ID:                    req.ID,
		Handle:                req.Handle,
		DisplayName:           req.DisplayName,
		OUID:                  req.OUID,
		OUHandle:              req.OUHandle,
		AllowSelfRegistration: req.AllowSelfRegistration,
		SystemAttributes:      req.SystemAttributes,
		Schema:                schemaBytes,
	}
	updateReq := entitytype.UpdateEntityTypeRequest{
		Handle:                createReq.Handle,
		DisplayName:           createReq.DisplayName,
		OUID:                  createReq.OUID,
		OUHandle:              createReq.OUHandle,
		AllowSelfRegistration: createReq.AllowSelfRegistration,
		SystemAttributes:      createReq.SystemAttributes,
		Schema:                createReq.Schema,
	}

	// The upsert looks the type up rather than attempting an update, so a create is not preceded by an
	// audited failed update.
	exists := false
	if options.IsUpsertEnabled() && req.ID != "" {
		_, svcErr := s.entityTypeService.GetEntityType(ctx, category, req.ID, false)
		if svcErr != nil && !isNotFoundServiceError(svcErr) {
			return serviceErrorOutcome(resourceTypeEntityType, req.ID, req.DisplayName, operationUpdate, svcErr)
		}
		exists = svcErr == nil
	}

	if dryRun {
		if exists {
			return successOutcome(resourceTypeEntityType, req.ID, req.DisplayName, operationUpdate)
		}
		return successOutcome(resourceTypeEntityType, req.ID, req.DisplayName, operationCreate)
	}

	if exists {
		updated, svcErr := s.entityTypeService.UpdateEntityType(ctx, category, req.ID, updateReq)
		if svcErr != nil {
			return serviceErrorOutcome(resourceTypeEntityType, req.ID, req.DisplayName, operationUpdate, svcErr)
		}
		return successOutcome(resourceTypeEntityType, updated.ID, updated.DisplayName, operationUpdate)
	}

	created, svcErr := s.entityTypeService.CreateEntityType(ctx, category, createReq)
	if svcErr != nil {
		return serviceErrorOutcome(resourceTypeEntityType, req.ID, req.DisplayName, operationCreate, svcErr)
	}
	return successOutcome(resourceTypeEntityType, created.ID, created.DisplayName, operationCreate)
}

func (s *importService) importRole(
	ctx context.Context, doc parsedDocument, options *ImportOptions, dryRun bool,
) ImportItemOutcome {
	if s.roleService == nil {
		return unsupportedAdapterOutcome(resourceTypeRole, "role")
	}

	var req roleDeclarativeYAML
	if err := doc.Node.Decode(&req); err != nil {
		return decodeErrorOutcome(resourceTypeRole, req.ID, req.Name, err)
	}

	resolvedOUID, svcErr := s.resolveImportOUHandle(
		ctx, resourceTypeRole, req.ID, req.Name, req.OUID, req.OUHandle)
	if svcErr != nil {
		return serviceErrorOutcome(resourceTypeRole, req.ID, req.Name, operationCreate, svcErr)
	}
	req.OUID = resolvedOUID

	// Assignments are added once the sharing policies are in place, since one may be made in an
	// organization unit only a policy lets the role reach.
	createReq := role.RoleCreationDetail{
		ID:          req.ID,
		Name:        req.Name,
		Description: req.Description,
		OUID:        req.OUID,
		Permissions: req.Permissions,
	}
	updateReq := role.RoleUpdateDetail{
		Name:        req.Name,
		Description: req.Description,
		OUID:        req.OUID,
		Permissions: req.Permissions,
	}

	if dryRun {
		if options.IsUpsertEnabled() && req.ID != "" {
			_, svcErr := s.roleService.GetRoleWithPermissions(ctx, req.ID)
			if svcErr == nil {
				return successOutcome(resourceTypeRole, req.ID, req.Name, operationUpdate)
			}

			if !isNotFoundServiceError(svcErr) {
				return serviceErrorOutcome(resourceTypeRole, req.ID, req.Name, operationUpdate, svcErr)
			}
		}

		return successOutcome(resourceTypeRole, req.ID, req.Name, operationCreate)
	}

	if options.IsUpsertEnabled() && req.ID != "" {
		_, svcErr := s.roleService.GetRoleWithPermissions(ctx, req.ID)
		if svcErr == nil {
			updated, updateErr := s.roleService.UpdateRoleWithPermissions(ctx, req.ID, updateReq)
			if updateErr != nil {
				return serviceErrorOutcome(resourceTypeRole, req.ID, req.Name, operationUpdate, updateErr)
			}
			if outcome := s.applySharingPolicies(ctx, role.RoleSharingType, resourceTypeRole,
				updated.ID, updated.Name, updated.OUID, req.SharingPolicies, operationUpdate); outcome != nil {
				return *outcome
			}
			if assignErr := s.addRoleAssignments(ctx, updated.ID, req.Assignments); assignErr != nil {
				return serviceErrorOutcome(resourceTypeRole, updated.ID, updated.Name, operationUpdate, assignErr)
			}
			return successOutcome(resourceTypeRole, updated.ID, updated.Name, operationUpdate)
		}

		if !isNotFoundServiceError(svcErr) {
			return serviceErrorOutcome(resourceTypeRole, req.ID, req.Name, operationUpdate, svcErr)
		}
	}

	created, svcErr := s.roleService.CreateRole(ctx, createReq)
	if svcErr != nil {
		return serviceErrorOutcome(resourceTypeRole, req.ID, req.Name, operationCreate, svcErr)
	}

	// A policy names the role it governs, so the policies are replayed once the role exists, and the
	// assignments after them. A refusal of either undoes the create, so a retry does not meet a
	// half-imported role.
	outcome := s.applySharingPolicies(ctx, role.RoleSharingType, resourceTypeRole,
		created.ID, created.Name, created.OUID, req.SharingPolicies, operationCreate)
	if outcome == nil {
		if assignErr := s.addRoleAssignments(ctx, created.ID, req.Assignments); assignErr != nil {
			refused := serviceErrorOutcome(resourceTypeRole, created.ID, created.Name, operationCreate, assignErr)
			outcome = &refused
		}
	}
	if outcome != nil {
		if delErr := s.roleService.DeleteRole(ctx, created.ID); delErr != nil {
			log.GetLogger().Error(ctx, "Failed to remove a role whose sharing policies or assignments were refused",
				log.String("roleID", created.ID), log.String("code", delErr.Code))
		}
		return *outcome
	}
	return successOutcome(resourceTypeRole, created.ID, created.Name, operationCreate)
}

// addRoleAssignments adds a role document's assignments, each in the organization unit it names or,
// when it names none, in the one owning its assignee.
func (s *importService) addRoleAssignments(
	ctx context.Context, roleID string, assignments []role.RoleAssignment,
) *tidcommon.ServiceError {
	if len(assignments) == 0 {
		return nil
	}
	if s.roleAssignmentService == nil {
		return tidcommon.CustomServiceError(tidcommon.InternalServerError,
			tidcommon.I18nMessage{DefaultValue: "roleAssignmentService not configured"})
	}
	return s.roleAssignmentService.AddAssigneesToRoles(ctx, assignments, []string{roleID})
}

func (s *importService) importGroup(
	ctx context.Context, doc parsedDocument, options *ImportOptions, dryRun bool,
) ImportItemOutcome {
	if s.groupService == nil {
		return unsupportedAdapterOutcome(resourceTypeGroup, "group")
	}

	var req group.CreateGroupRequest
	// Use a local struct to capture the ID from YAML (ID is json:"-" on CreateGroupRequest)
	var raw struct {
		ID          string         `yaml:"id"`
		Name        string         `yaml:"name"`
		Description string         `yaml:"description,omitempty"`
		OUID        string         `yaml:"ouId,omitempty"`
		OUHandle    string         `yaml:"ouHandle,omitempty"`
		Members     []group.Member `yaml:"members,omitempty"`
	}
	if err := doc.Node.Decode(&raw); err != nil {
		return decodeErrorOutcome(resourceTypeGroup, raw.ID, raw.Name, err)
	}

	resolvedOUID, svcErr := s.resolveImportOUHandle(
		ctx, resourceTypeGroup, raw.ID, raw.Name, raw.OUID, raw.OUHandle)
	if svcErr != nil {
		return serviceErrorOutcome(resourceTypeGroup, raw.ID, raw.Name, operationCreate, svcErr)
	}
	raw.OUID = resolvedOUID

	req = group.CreateGroupRequest{
		ID:          raw.ID,
		Name:        raw.Name,
		Description: raw.Description,
		OUID:        raw.OUID,
	}

	updateReq := group.UpdateGroupRequest{
		Name:        raw.Name,
		Description: raw.Description,
		OUID:        raw.OUID,
	}

	if dryRun {
		if options.IsUpsertEnabled() && raw.ID != "" {
			_, svcErr := s.groupService.GetGroup(ctx, raw.ID, false)
			if svcErr == nil {
				return successOutcome(resourceTypeGroup, raw.ID, raw.Name, operationUpdate)
			}
			if !isNotFoundServiceError(svcErr) {
				return serviceErrorOutcome(resourceTypeGroup, raw.ID, raw.Name, operationUpdate, svcErr)
			}
		}
		return successOutcome(resourceTypeGroup, raw.ID, raw.Name, operationCreate)
	}

	if options.IsUpsertEnabled() && raw.ID != "" {
		_, svcErr := s.groupService.GetGroup(ctx, raw.ID, false)
		if svcErr == nil {
			updated, updateErr := s.groupService.UpdateGroup(ctx, raw.ID, updateReq)
			if updateErr != nil {
				return serviceErrorOutcome(resourceTypeGroup, raw.ID, raw.Name, operationUpdate, updateErr)
			}
			if len(raw.Members) > 0 {
				if _, memberErr := s.groupService.AddGroupMembers(ctx, updated.ID, raw.Members); memberErr != nil {
					return serviceErrorOutcome(resourceTypeGroup, updated.ID, updated.Name, operationUpdate, memberErr)
				}
			}
			return successOutcome(resourceTypeGroup, updated.ID, updated.Name, operationUpdate)
		}
		if !isNotFoundServiceError(svcErr) {
			return serviceErrorOutcome(resourceTypeGroup, raw.ID, raw.Name, operationUpdate, svcErr)
		}
	}

	grp, svcErr := s.groupService.CreateGroup(ctx, req)
	if svcErr != nil {
		return serviceErrorOutcome(resourceTypeGroup, raw.ID, raw.Name, operationCreate, svcErr)
	}
	if len(raw.Members) > 0 {
		if _, memberErr := s.groupService.AddGroupMembers(ctx, grp.ID, raw.Members); memberErr != nil {
			return serviceErrorOutcome(resourceTypeGroup, grp.ID, grp.Name, operationCreate, memberErr)
		}
	}
	return successOutcome(resourceTypeGroup, grp.ID, grp.Name, operationCreate)
}

func (s *importService) importResourceServer(
	ctx context.Context, doc parsedDocument, options *ImportOptions, dryRun bool,
) ImportItemOutcome {
	if s.resourceService == nil {
		return unsupportedAdapterOutcome(resourceTypeResourceServer, "resource server")
	}

	var req providers.ResourceServer
	if err := doc.Node.Decode(&req); err != nil {
		return decodeErrorOutcome(resourceTypeResourceServer, req.ID, req.Name, err)
	}

	resolvedOUID, svcErr := s.resolveImportOUHandle(
		ctx, resourceTypeResourceServer, req.ID, req.Name, req.OUID, req.OUHandle)
	if svcErr != nil {
		return serviceErrorOutcome(resourceTypeResourceServer, req.ID, req.Name, operationCreate, svcErr)
	}
	req.OUID = resolvedOUID

	if dryRun {
		if options.IsUpsertEnabled() && req.ID != "" {
			_, svcErr := s.resourceService.GetResourceServer(ctx, req.ID)
			if svcErr == nil {
				return successOutcome(resourceTypeResourceServer, req.ID, req.Name, operationUpdate)
			}

			if !isNotFoundServiceError(svcErr) {
				return serviceErrorOutcome(resourceTypeResourceServer, req.ID, req.Name, operationUpdate, svcErr)
			}
		}

		return successOutcome(resourceTypeResourceServer, req.ID, req.Name, operationCreate)
	}

	if options.IsUpsertEnabled() && req.ID != "" {
		updated, svcErr := s.resourceService.UpdateResourceServer(ctx, req.ID, req)
		if svcErr == nil {
			if err := s.importResourceServerChildren(ctx, updated.ID, req); err != nil {
				return serviceErrorOutcome(resourceTypeResourceServer, updated.ID, updated.Name, operationUpdate, err)
			}
			return successOutcome(resourceTypeResourceServer, updated.ID, updated.Name, operationUpdate)
		}

		if !isNotFoundServiceError(svcErr) {
			return serviceErrorOutcome(resourceTypeResourceServer, req.ID, req.Name, operationUpdate, svcErr)
		}
	}

	created, svcErr := s.resourceService.CreateResourceServer(ctx, req)
	if svcErr != nil {
		return serviceErrorOutcome(resourceTypeResourceServer, req.ID, req.Name, operationCreate, svcErr)
	}

	if err := s.importResourceServerChildren(ctx, created.ID, req); err != nil {
		return serviceErrorOutcome(resourceTypeResourceServer, created.ID, created.Name, operationCreate, err)
	}

	return successOutcome(resourceTypeResourceServer, created.ID, created.Name, operationCreate)
}

//nolint:dupl // Theme and layout imports share the same upsert pattern with type-specific services.
func (s *importService) importTheme(
	ctx context.Context, doc parsedDocument, options *ImportOptions, dryRun bool) ImportItemOutcome {
	if s.themeService == nil {
		return unsupportedAdapterOutcome(resourceTypeTheme, "theme")
	}

	var req themeDeclarativeYAML
	if err := doc.Node.Decode(&req); err != nil {
		return decodeErrorOutcome(resourceTypeTheme, req.ID, req.DisplayName, err)
	}

	themeBytes, err := json.Marshal(req.Theme)
	if err != nil {
		return ImportItemOutcome{
			ResourceType: resourceTypeTheme,
			ResourceID:   req.ID,
			ResourceName: req.DisplayName,
			Status:       statusFailed,
			Code:         ErrorInvalidYAMLContent.Code,
			Message:      fmt.Sprintf("failed to marshal theme: %v", err),
		}
	}

	createReq := thememgt.CreateThemeRequestWithID{
		ID:          req.ID,
		Handle:      req.Handle,
		DisplayName: req.DisplayName,
		Description: req.Description,
		Theme:       themeBytes,
	}
	updateReq := thememgt.UpdateThemeRequest{
		Handle:      req.Handle,
		DisplayName: req.DisplayName,
		Description: req.Description,
		Theme:       themeBytes,
	}

	// The upsert looks the theme up rather than attempting an update, so a create is not preceded by an
	// audited failed update.
	exists := false
	if options.IsUpsertEnabled() && req.ID != "" {
		_, svcErr := s.themeService.GetTheme(ctx, req.ID)
		if svcErr != nil && !isNotFoundServiceError(svcErr) {
			return serviceErrorOutcome(resourceTypeTheme, req.ID, req.DisplayName, operationUpdate, svcErr)
		}
		exists = svcErr == nil
	}

	if dryRun {
		if exists {
			return successOutcome(resourceTypeTheme, req.ID, req.DisplayName, operationUpdate)
		}
		return successOutcome(resourceTypeTheme, req.ID, req.DisplayName, operationCreate)
	}

	if exists {
		updated, svcErr := s.themeService.UpdateTheme(ctx, req.ID, updateReq)
		if svcErr != nil {
			return serviceErrorOutcome(resourceTypeTheme, req.ID, req.DisplayName, operationUpdate, svcErr)
		}
		return successOutcome(resourceTypeTheme, updated.ID, updated.DisplayName, operationUpdate)
	}

	created, svcErr := s.themeService.CreateTheme(ctx, createReq)
	if svcErr != nil {
		return serviceErrorOutcome(resourceTypeTheme, req.ID, req.DisplayName, operationCreate, svcErr)
	}

	return successOutcome(resourceTypeTheme, created.ID, created.DisplayName, operationCreate)
}

//nolint:dupl // Theme and layout imports share the same upsert pattern with type-specific services.
func (s *importService) importLayout(
	ctx context.Context, doc parsedDocument, options *ImportOptions, dryRun bool) ImportItemOutcome {
	if s.layoutService == nil {
		return unsupportedAdapterOutcome(resourceTypeLayout, "layout")
	}

	var req layoutDeclarativeYAML
	if err := doc.Node.Decode(&req); err != nil {
		return decodeErrorOutcome(resourceTypeLayout, req.ID, req.DisplayName, err)
	}

	layoutBytes, err := json.Marshal(req.Layout)
	if err != nil {
		return ImportItemOutcome{
			ResourceType: resourceTypeLayout,
			ResourceID:   req.ID,
			ResourceName: req.DisplayName,
			Status:       statusFailed,
			Code:         ErrorInvalidYAMLContent.Code,
			Message:      fmt.Sprintf("failed to marshal layout: %v", err),
		}
	}

	createReq := layoutmgt.CreateLayoutRequestWithID{
		ID:          req.ID,
		Handle:      req.Handle,
		DisplayName: req.DisplayName,
		Description: req.Description,
		Layout:      layoutBytes,
	}
	updateReq := layoutmgt.UpdateLayoutRequest{
		Handle:      req.Handle,
		DisplayName: req.DisplayName,
		Description: req.Description,
		Layout:      layoutBytes,
	}

	// The upsert looks the layout up rather than attempting an update, so a create is not preceded by an
	// audited failed update.
	exists := false
	if options.IsUpsertEnabled() && req.ID != "" {
		_, svcErr := s.layoutService.GetLayout(ctx, req.ID)
		if svcErr != nil && !isNotFoundServiceError(svcErr) {
			return serviceErrorOutcome(resourceTypeLayout, req.ID, req.DisplayName, operationUpdate, svcErr)
		}
		exists = svcErr == nil
	}

	if dryRun {
		if exists {
			return successOutcome(resourceTypeLayout, req.ID, req.DisplayName, operationUpdate)
		}
		return successOutcome(resourceTypeLayout, req.ID, req.DisplayName, operationCreate)
	}

	if exists {
		updated, svcErr := s.layoutService.UpdateLayout(ctx, req.ID, updateReq)
		if svcErr != nil {
			return serviceErrorOutcome(resourceTypeLayout, req.ID, req.DisplayName, operationUpdate, svcErr)
		}
		return successOutcome(resourceTypeLayout, updated.ID, updated.DisplayName, operationUpdate)
	}

	created, svcErr := s.layoutService.CreateLayout(ctx, createReq)
	if svcErr != nil {
		return serviceErrorOutcome(resourceTypeLayout, req.ID, req.DisplayName, operationCreate, svcErr)
	}

	return successOutcome(resourceTypeLayout, created.ID, created.DisplayName, operationCreate)
}

// credentialsToWrite is a document's credentials as the user service takes them, or nil when there
// are none to write. For an update, a credential the document carries no value for is left out.
func credentialsToWrite(credentials map[string]interface{}, update bool) (json.RawMessage, error) {
	kept := make(map[string]interface{}, len(credentials))
	for name, value := range credentials {
		if text, ok := value.(string); ok && text == "" && update {
			continue
		}
		kept[name] = value
	}
	if len(kept) == 0 {
		return nil, nil
	}
	encoded, err := json.Marshal(kept)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

// credentialRollbackError reports a resource whose credentials could not be written after it was
// created, and which could not be deleted again either.
func credentialRollbackError(resourceType, resourceID string,
	credErr, rollbackErr *tidcommon.ServiceError) *tidcommon.ServiceError {
	combined := *credErr
	combined.Error.DefaultValue = fmt.Sprintf("%s credential update failed: %s; rollback delete failed: %s",
		resourceType, credErr.Error.DefaultValue, rollbackErr.Error.DefaultValue)
	combined.ErrorDescription.DefaultValue = fmt.Sprintf(
		"credential update error code %s for %s %s; rollback delete error code %s",
		credErr.Code, resourceType, resourceID, rollbackErr.Code)
	return &combined
}

// userSharingReplay is a user's sharing policies that mandate roles, replayed once the import's roles are in.
type userSharingReplay struct {
	// resultIndex is the user's outcome, which a refusal replaces.
	resultIndex int
	userID      string
	owningOUID  string
	policies    []sharing.PolicyRequest
	operation   string
}

// importUser imports a user, deferring the sharing policies that mandate roles to mandating.
func (s *importService) importUser(
	ctx context.Context, doc parsedDocument, options *ImportOptions, dryRun bool, mandating *[]userSharingReplay,
) ImportItemOutcome {
	if s.userService == nil {
		return unsupportedAdapterOutcome(resourceTypeUser, "user")
	}

	var req userDeclarativeYAML
	if err := doc.Node.Decode(&req); err != nil {
		return decodeErrorOutcome(resourceTypeUser, req.ID, "", err)
	}

	resolvedOUID, svcErr := s.resolveImportOUHandle(
		ctx, resourceTypeUser, req.ID, "", req.OUID, req.OUHandle)
	if svcErr != nil {
		return serviceErrorOutcome(resourceTypeUser, req.ID, "", operationCreate, svcErr)
	}
	req.OUID = resolvedOUID

	attributesJSON, err := json.Marshal(req.Attributes)
	if err != nil {
		return ImportItemOutcome{ResourceType: resourceTypeUser, ResourceID: req.ID, Status: statusFailed,
			Code: ErrorInvalidYAMLContent.Code, Message: fmt.Sprintf("failed to marshal user attributes: %v", err)}
	}

	userReq := &providers.User{
		ID:         req.ID,
		OUID:       req.OUID,
		Type:       req.Type,
		Attributes: attributesJSON,
	}

	credentialsJSON, err := credentialsToWrite(req.Credentials, false)
	if err != nil {
		return ImportItemOutcome{ResourceType: resourceTypeUser, ResourceID: req.ID, Status: statusFailed,
			Code: ErrorInvalidYAMLContent.Code, Message: fmt.Sprintf("failed to marshal user credentials: %v", err)}
	}
	// A credential the document names but carries no value for is left as the user has it: a stored
	// credential is a hash that never leaves an export, so a server importing its own export, or one
	// that fills the placeholder with nothing, has nothing new to set. A new user is different:
	// without a credential they could never sign in, so an empty one is refused on a create.
	credentialsToKeep, err := credentialsToWrite(req.Credentials, true)
	if err != nil {
		return ImportItemOutcome{ResourceType: resourceTypeUser, ResourceID: req.ID, Status: statusFailed,
			Code: ErrorInvalidYAMLContent.Code, Message: fmt.Sprintf("failed to marshal user credentials: %v", err)}
	}

	// The upsert looks the user up rather than attempting an update, so a create is not preceded by an
	// audited failed update.
	exists := false
	if options.IsUpsertEnabled() && req.ID != "" {
		_, svcErr := s.userService.GetUser(ctx, req.ID, false)
		if svcErr != nil && !isNotFoundServiceError(svcErr) {
			return serviceErrorOutcome(resourceTypeUser, req.ID, "", operationUpdate, svcErr)
		}
		exists = svcErr == nil
	}

	if dryRun {
		if exists {
			return successOutcome(resourceTypeUser, req.ID, "", operationUpdate)
		}
		return successOutcome(resourceTypeUser, req.ID, "", operationCreate)
	}

	if exists {
		updated, svcErr := s.userService.UpdateUser(ctx, req.ID, userReq)
		if svcErr == nil {
			if credentialsToKeep != nil {
				if credErr := s.userService.UpdateUserCredentials(
					ctx,
					req.ID,
					credentialsToKeep,
				); credErr != nil {
					// Profile is already committed; emit a clear partial-failure outcome.
					return ImportItemOutcome{
						ResourceType: resourceTypeUser,
						ResourceID:   req.ID,
						Operation:    operationUpdate,
						Status:       statusFailed,
						Code:         credErr.Code,
						Message: "user profile updated but credential update failed: " +
							credErr.Error.DefaultValue,
					}
				}
			}
			// Replaying such policies without their roles first would strip the mandates of a share that is already
			// there, so the whole replay waits.
			if mandatesRoles(req.SharingPolicies) {
				*mandating = append(*mandating, userSharingReplay{userID: updated.ID, owningOUID: updated.OUID,
					policies: req.SharingPolicies, operation: operationUpdate})
				return successOutcome(resourceTypeUser, updated.ID, "", operationUpdate)
			}
			if outcome := s.applySharingPolicies(ctx, user.UserSharingType, resourceTypeUser,
				updated.ID, "", updated.OUID, req.SharingPolicies, operationUpdate); outcome != nil {
				return *outcome
			}
			return successOutcome(resourceTypeUser, updated.ID, "", operationUpdate)
		}
		return serviceErrorOutcome(resourceTypeUser, req.ID, "", operationUpdate, svcErr)
	}

	created, svcErr := s.userService.CreateUser(ctx, userReq)
	if svcErr != nil {
		return serviceErrorOutcome(resourceTypeUser, req.ID, "", operationCreate, svcErr)
	}
	if credentialsJSON != nil {
		if credErr := s.userService.UpdateUserCredentials(
			ctx,
			created.ID,
			credentialsJSON,
		); credErr != nil {
			if rollbackErr := s.userService.DeleteUser(ctx, created.ID); rollbackErr != nil {
				return serviceErrorOutcome(resourceTypeUser, created.ID, "", operationCreate,
					credentialRollbackError(resourceTypeUser, created.ID, credErr, rollbackErr))
			}

			return serviceErrorOutcome(resourceTypeUser, created.ID, "", operationCreate, credErr)
		}
	}

	// The policies come after the user exists, because a policy names the user it governs. A refusal
	// therefore undoes the create, so a retry does not meet a half-imported user. Policies mandating roles are
	// replayed now without them, so the user already reaches its units when a role later in the import is
	// assigned to it there, and in full once the roles are in.
	policies := req.SharingPolicies
	deferred := mandatesRoles(policies)
	if deferred {
		policies = withoutMandatedRoles(policies)
	}
	if outcome := s.applySharingPolicies(ctx, user.UserSharingType, resourceTypeUser,
		created.ID, "", created.OUID, policies, operationCreate); outcome != nil {
		s.undoUserCreate(ctx, created.ID)
		return *outcome
	}
	if deferred {
		*mandating = append(*mandating, userSharingReplay{userID: created.ID, owningOUID: created.OUID,
			policies: req.SharingPolicies, operation: operationCreate})
	}

	return successOutcome(resourceTypeUser, created.ID, "", operationCreate)
}

// replayMandatingUserSharing replays the role-mandating user shares after the roles, returning how many failed.
func (s *importService) replayMandatingUserSharing(
	ctx context.Context, mandating []userSharingReplay, results []ImportItemOutcome,
) int {
	refused := 0
	for _, r := range mandating {
		outcome := s.applySharingPolicies(ctx, user.UserSharingType, resourceTypeUser,
			r.userID, "", r.owningOUID, r.policies, r.operation)
		if outcome == nil {
			continue
		}
		if r.operation == operationCreate {
			s.undoUserCreate(ctx, r.userID)
		}
		results[r.resultIndex] = *outcome
		refused++
	}
	return refused
}

// undoUserCreate removes a user whose sharing policies were refused.
func (s *importService) undoUserCreate(ctx context.Context, userID string) {
	if delErr := s.userService.DeleteUser(ctx, userID); delErr != nil {
		log.GetLogger().Error(ctx, "Failed to remove a user whose sharing policies were refused",
			log.MaskedString(log.LoggerKeyUserID, userID), log.String("code", delErr.Code))
	}
}

// mandatesRoles reports whether any of a user's sharing policies carries a roles rule.
func mandatesRoles(policies []sharing.PolicyRequest) bool {
	for _, p := range policies {
		for _, rules := range overlayRuleSets(p) {
			if _, ok := rules[user.RolesFieldKey]; ok {
				return true
			}
		}
	}
	return false
}

// overlayRuleSets lists every set of overlay rules a policy carries, section by section and unit by unit.
func overlayRuleSets(p sharing.PolicyRequest) []map[string]sharing.OverlayRule {
	var sets []map[string]sharing.OverlayRule
	if p.AllUnits != nil {
		sets = append(sets, p.AllUnits.OverlayRules)
	}
	if p.ChildUnits != nil {
		sets = append(sets, p.ChildUnits.OverlayRules)
		for _, u := range p.ChildUnits.Units {
			sets = append(sets, u.OverlayRules)
		}
	}
	if p.OtherTreeRoots != nil {
		sets = append(sets, p.OtherTreeRoots.OverlayRules)
		for _, u := range p.OtherTreeRoots.Units {
			sets = append(sets, u.OverlayRules)
		}
	}
	return sets
}

// withoutMandatedRoles returns copies of the policies with every roles rule left out.
func withoutMandatedRoles(policies []sharing.PolicyRequest) []sharing.PolicyRequest {
	out := make([]sharing.PolicyRequest, len(policies))
	for i, p := range policies {
		if p.AllUnits != nil {
			all := *p.AllUnits
			all.OverlayRules = withoutRolesRule(all.OverlayRules)
			p.AllUnits = &all
		}
		if p.ChildUnits != nil {
			child := *p.ChildUnits
			child.OverlayRules = withoutRolesRule(child.OverlayRules)
			child.Units = slices.Clone(child.Units)
			for j := range child.Units {
				child.Units[j].OverlayRules = withoutRolesRule(child.Units[j].OverlayRules)
			}
			p.ChildUnits = &child
		}
		if p.OtherTreeRoots != nil {
			roots := *p.OtherTreeRoots
			roots.OverlayRules = withoutRolesRule(roots.OverlayRules)
			roots.Units = slices.Clone(roots.Units)
			for j := range roots.Units {
				roots.Units[j].OverlayRules = withoutRolesRule(roots.Units[j].OverlayRules)
			}
			p.OtherTreeRoots = &roots
		}
		out[i] = p
	}
	return out
}

// withoutRolesRule returns rules less the roles rule, leaving the given map unchanged.
func withoutRolesRule(rules map[string]sharing.OverlayRule) map[string]sharing.OverlayRule {
	if _, ok := rules[user.RolesFieldKey]; !ok {
		return rules
	}
	out := maps.Clone(rules)
	delete(out, user.RolesFieldKey)
	return out
}

func (s *importService) importTranslation(ctx context.Context, doc parsedDocument, dryRun bool) ImportItemOutcome {
	if s.translationService == nil {
		return unsupportedAdapterOutcome(resourceTypeTranslation, "translation")
	}

	var req i18nmgt.LanguageTranslations
	if err := doc.Node.Decode(&req); err != nil {
		return decodeErrorOutcome(resourceTypeTranslation, "", req.Language, err)
	}

	if dryRun {
		return successOutcome(resourceTypeTranslation, "", req.Language, operationUpdate)
	}

	_, i18nErr := s.translationService.SetTranslationOverrides(ctx, req.Language, req.Translations)
	if i18nErr != nil {
		return ImportItemOutcome{
			ResourceType: resourceTypeTranslation,
			ResourceName: req.Language,
			Operation:    operationUpdate,
			Status:       statusFailed,
			Code:         i18nErr.Code,
			Message:      i18nErr.Error.DefaultValue,
		}
	}

	return successOutcome(resourceTypeTranslation, "", req.Language, operationUpdate)
}

// importResourceServerChildren creates the actions of a resource server and the resources and actions
// nested under it. It first computes permission strings via ProcessResourceServer, then calls the
// resource service for each resource and action.  Existing resources/actions (on upsert paths) are
// silently skipped.
func (s *importService) importResourceServerChildren(
	ctx context.Context, serverID string, rs providers.ResourceServer,
) *tidcommon.ServiceError {
	if len(rs.Actions) == 0 && len(rs.Resources) == 0 {
		return nil
	}

	// Compute permission strings in-place (mirrors declarative loader logic).
	if err := resource.ProcessResourceServer(&rs); err != nil {
		return &tidcommon.ServiceError{
			Code: ErrorInvalidYAMLContent.Code,
			Type: ErrorInvalidYAMLContent.Type,
			Error: tidcommon.I18nMessage{
				DefaultValue: fmt.Sprintf("failed to process resource server children: %v", err),
			},
		}
	}

	for j := range rs.Actions {
		if svcErr := s.importResourceServerAction(ctx, serverID, nil, rs.Actions[j]); svcErr != nil {
			return svcErr
		}
	}

	// pathToID maps a resource path (its permission) → created/resolved ID for parent resolution.
	pathToID := make(map[string]string)
	// existingIDs maps a parent ID, empty for the top level, to the handles of the resources that
	// already exist under it and their IDs. A parent's resources are listed on the first handle
	// conflict under it, so an upsert re-import lists each parent's resources only once.
	existingIDs := make(map[string]map[string]string)

	// A child is written after its parent, whichever order the document lists them in, so its
	// parent path always resolves to an id already created or found. An export lists resources
	// newest first, which puts a child before its parent.
	ordered := parentsFirst(rs.Resources)
	for i := range ordered {
		res := ordered[i]

		// Resolve the parent path to the parent ID using resources seen so far in this import.
		if res.ParentHandle != "" {
			if parentID, ok := pathToID[res.ParentHandle]; ok {
				res.Parent = &parentID
			}
		}

		created, svcErr := s.resourceService.CreateResource(ctx, serverID, res)
		if svcErr != nil {
			if svcErr.Code != resource.ErrorHandleConflict.Code {
				return svcErr
			}
			// Resource already exists — look it up under the same parent scope to get its ID.
			var parentID *string
			if res.ParentHandle != "" {
				if pid, ok := pathToID[res.ParentHandle]; ok {
					parentID = &pid
				}
			}
			parentKey := ""
			if parentID != nil {
				parentKey = *parentID
			}
			handleToID, listed := existingIDs[parentKey]
			if !listed {
				handleToID = make(map[string]string)
				for offset := 0; ; offset += serverconst.MaxPageSize {
					list, listErr := s.resourceService.GetResourceList(
						ctx, serverID, parentID, serverconst.MaxPageSize, offset)
					if listErr != nil {
						return listErr
					}
					for j := range list.Resources {
						handleToID[list.Resources[j].Handle] = list.Resources[j].ID
					}
					if len(list.Resources) < serverconst.MaxPageSize {
						break
					}
				}
				existingIDs[parentKey] = handleToID
			}
			existingID, ok := handleToID[res.Handle]
			if !ok {
				continue
			}
			created = &providers.Resource{ID: existingID}
		}

		pathToID[res.Permission] = created.ID

		for j := range res.Actions {
			if svcErr := s.importResourceServerAction(ctx, serverID, &created.ID, res.Actions[j]); svcErr != nil {
				return svcErr
			}
		}
	}

	return nil
}

// importResourceServerAction creates an action of a resource, or of the resource server itself when
// resourceID is nil. An action whose handle already exists there is skipped.
func (s *importService) importResourceServerAction(
	ctx context.Context, serverID string, resourceID *string, action providers.Action,
) *tidcommon.ServiceError {
	_, svcErr := s.resourceService.CreateAction(ctx, serverID, resourceID, action)
	if svcErr != nil && svcErr.Code != resource.ErrorHandleConflict.Code {
		return svcErr
	}
	return nil
}

// parentsFirst orders resources so each comes after the resource its parent path names. A parent
// the document does not hold, or a cycle, which ProcessResourceServer refuses before this, leaves
// the resource where it is rather than dropping it. Resources are tracked by position rather than by
// path, so two sharing a path, which ProcessResourceServer also refuses, are both kept and the
// ordering still ends.
func parentsFirst(resources []providers.Resource) []providers.Resource {
	held := make(map[string]bool, len(resources))
	for _, res := range resources {
		held[res.Permission] = true
	}
	ordered := make([]providers.Resource, 0, len(resources))
	placed := make([]bool, len(resources))
	placedPaths := make(map[string]bool, len(resources))
	for len(ordered) < len(resources) {
		progressed := false
		for i, res := range resources {
			if placed[i] {
				continue
			}
			if res.ParentHandle == "" || placedPaths[res.ParentHandle] || !held[res.ParentHandle] {
				ordered = append(ordered, res)
				placed[i] = true
				placedPaths[res.Permission] = true
				progressed = true
			}
		}
		if !progressed {
			for i, res := range resources {
				if !placed[i] {
					ordered = append(ordered, res)
					placed[i] = true
				}
			}
		}
	}
	return ordered
}

func (s *importService) importAgent(
	ctx context.Context, doc parsedDocument, options *ImportOptions, dryRun bool, flowIDAliases map[string]string,
) ImportItemOutcome {
	if s.agentService == nil {
		return unsupportedAdapterOutcome(resourceTypeAgent, "agent")
	}

	var req agentmodel.AgentRequestWithID
	if err := doc.Node.Decode(&req); err != nil {
		return decodeErrorOutcome(resourceTypeAgent, req.ID, "", err)
	}

	if mappedFlowID, ok := flowIDAliases[req.AuthFlowID]; ok {
		req.AuthFlowID = mappedFlowID
	}
	if mappedFlowID, ok := flowIDAliases[req.RegistrationFlowID]; ok {
		req.RegistrationFlowID = mappedFlowID
	}

	var attributesJSON json.RawMessage
	if len(req.Attributes) > 0 {
		raw, err := json.Marshal(req.Attributes)
		if err != nil {
			return ImportItemOutcome{
				ResourceType: resourceTypeAgent,
				ResourceID:   req.ID,
				Status:       statusFailed,
				Code:         ErrorInvalidYAMLContent.Code,
				Message:      fmt.Sprintf("failed to marshal agent attributes: %v", err),
			}
		}
		attributesJSON = raw
	}

	normalizeAgentOAuthConfigForImport(ctx, &req)

	// Credentials are written after the agent, as a user's are, so a document's credentials are held
	// to the ones the agent's type declares.
	credentialsJSON, err := json.Marshal(req.Credentials)
	if err != nil {
		return ImportItemOutcome{ResourceType: resourceTypeAgent, ResourceID: req.ID, Status: statusFailed,
			Code: ErrorInvalidYAMLContent.Code, Message: fmt.Sprintf("failed to marshal agent credentials: %v", err)}
	}

	createReq := &providers.Agent{
		ID:          req.ID,
		OUID:        req.OUID,
		OUHandle:    req.OUHandle,
		ProjectID:   req.ProjectID,
		Type:        req.Type,
		Description: req.Description,
		LogoURL:     req.LogoURL,
		Owner:       req.Owner,
		Attributes:  attributesJSON,
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID:                req.AuthFlowID,
			AuthFlowHandle:            req.AuthFlowHandle,
			RegistrationFlowID:        req.RegistrationFlowID,
			RegistrationFlowHandle:    req.RegistrationFlowHandle,
			IsRegistrationFlowEnabled: req.IsRegistrationFlowEnabled,
			RecoveryFlowID:            req.RecoveryFlowID,
			RecoveryFlowHandle:        req.RecoveryFlowHandle,
			IsRecoveryFlowEnabled:     req.IsRecoveryFlowEnabled,
			SignOutFlowID:             req.SignOutFlowID,
			SignOutFlowHandle:         req.SignOutFlowHandle,
			ThemeID:                   req.ThemeID,
			LayoutID:                  req.LayoutID,
			Assertion:                 req.Assertion,
			LoginConsent:              req.LoginConsent,
			AllowedUserTypes:          req.AllowedUserTypes,
			AllowedAgentTypes:         req.AllowedAgentTypes,
			PasskeyAllowedOrigins:     req.PasskeyAllowedOrigins,
			Attestation:               req.Attestation,
		},
		InboundAuthConfig: req.InboundAuthConfig,
	}
	updateReq := &agentmodel.UpdateAgentRequest{
		OUID:                  req.OUID,
		OUHandle:              req.OUHandle,
		ProjectID:             req.ProjectID,
		Type:                  req.Type,
		Description:           req.Description,
		LogoURL:               req.LogoURL,
		Owner:                 req.Owner,
		Attributes:            attributesJSON,
		InboundAuthProfileReq: req.InboundAuthProfileReq,
		InboundAuthConfig:     req.InboundAuthConfig,
	}
	// An import restores the lifespan as exported, an expired one included; an absent one leaves an
	// existing agent's unchanged. A malformed value is reported by the dry run as well.
	if req.ValidUntil != "" {
		if svcErr := agent.ValidateLifespanFormat(req.ValidUntil); svcErr != nil {
			return serviceErrorOutcome(resourceTypeAgent, req.ID, "", operationCreate, svcErr)
		}
		validUntil := req.ValidUntil
		rawValidUntil, _ := json.Marshal(validUntil)
		createReq.ValidUntil = &validUntil
		updateReq.ValidUntil = rawValidUntil
	}
	ctx = agent.WithLifespanRestore(ctx)

	if dryRun {
		if options.IsUpsertEnabled() && req.ID != "" {
			_, svcErr := s.agentService.GetAgent(ctx, req.ID, false)
			if svcErr == nil {
				return successOutcome(resourceTypeAgent, req.ID, "", operationUpdate)
			}

			if !isNotFoundServiceError(svcErr) {
				return serviceErrorOutcome(resourceTypeAgent, req.ID, "", operationUpdate, svcErr)
			}
		}

		return successOutcome(resourceTypeAgent, req.ID, "", operationCreate)
	}

	if options.IsUpsertEnabled() && req.ID != "" {
		_, svcErr := s.agentService.GetAgent(ctx, req.ID, false)
		if svcErr == nil {
			updated, updateErr := s.agentService.UpdateAgent(ctx, req.ID, updateReq)
			if updateErr != nil {
				return serviceErrorOutcome(resourceTypeAgent, req.ID, "", operationUpdate, updateErr)
			}
			if len(req.Credentials) > 0 {
				if credErr := s.agentService.UpdateAgentCredentials(ctx, req.ID, credentialsJSON); credErr != nil {
					// The profile is already committed, so this is a partial failure.
					return ImportItemOutcome{
						ResourceType: resourceTypeAgent,
						ResourceID:   req.ID,
						Operation:    operationUpdate,
						Status:       statusFailed,
						Code:         credErr.Code,
						Message: "agent profile updated but credential update failed: " +
							credErr.Error.DefaultValue,
					}
				}
			}
			return successOutcome(resourceTypeAgent, updated.ID, "", operationUpdate)
		}

		if !isNotFoundServiceError(svcErr) {
			return serviceErrorOutcome(resourceTypeAgent, req.ID, "", operationUpdate, svcErr)
		}
	}

	created, svcErr := s.agentService.CreateAgent(ctx, createReq)
	if svcErr != nil {
		return serviceErrorOutcome(resourceTypeAgent, req.ID, "", operationCreate, svcErr)
	}
	if len(req.Credentials) > 0 {
		if credErr := s.agentService.UpdateAgentCredentials(ctx, created.ID, credentialsJSON); credErr != nil {
			if rollbackErr := s.agentService.DeleteAgent(ctx, created.ID); rollbackErr != nil {
				return serviceErrorOutcome(resourceTypeAgent, created.ID, "", operationCreate,
					credentialRollbackError(resourceTypeAgent, created.ID, credErr, rollbackErr))
			}
			return serviceErrorOutcome(resourceTypeAgent, created.ID, "", operationCreate, credErr)
		}
	}
	return successOutcome(resourceTypeAgent, created.ID, "", operationCreate)
}

func getAgentOAuthConfigForImport(req *agentmodel.AgentRequestWithID) *providers.OAuthConfigWithSecret {
	if req == nil {
		return nil
	}

	for _, inboundAuth := range req.InboundAuthConfig {
		if inboundAuth.Type == providers.OAuthInboundAuthType && inboundAuth.OAuthConfig != nil {
			return inboundAuth.OAuthConfig
		}
	}

	return nil
}

func normalizeAgentOAuthConfigForImport(ctx context.Context, req *agentmodel.AgentRequestWithID) {
	oauthConfig := getAgentOAuthConfigForImport(req)
	if oauthConfig == nil {
		return
	}

	if dropUnusedClientSecret(oauthConfig) {
		log.GetLogger().Debug(ctx, "Dropping client_secret for agent import that does not use a client secret",
			log.String("agentID", req.ID),
			log.String("clientID", oauthConfig.ClientID),
			log.String("tokenEndpointAuthMethod", string(oauthConfig.TokenEndpointAuthMethod)))
	}
}

// applySharingPolicies replays the sharing policies a document carries for one resource, the way
// role assignments and resource server children are applied: by the importer, after the resource
// itself is written, rather than by the resource's own create and update.
func (s *importService) applySharingPolicies(
	ctx context.Context, rt sharing.ResourceType, resourceType, resourceID, resourceName, owningOUID string,
	policies []sharing.PolicyRequest, operation string,
) *ImportItemOutcome {
	if len(policies) == 0 {
		return nil
	}
	if s.sharingService == nil {
		outcome := unsupportedAdapterOutcome(resourceType, "sharing")
		return &outcome
	}

	replay, replayErr := s.sharingService.ReplacePoliciesForResource(ctx, rt, resourceID, owningOUID, policies)
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ImportService"))
	for _, skipped := range replay.Skipped {
		logger.Warn(ctx, "Sharing policy in the document was skipped; a declared policy governs this unit",
			log.String("resourceType", resourceType), log.String("resourceID", resourceID),
			log.Int("policyIndex", skipped.Index+1),
			log.String("initiatingOUID", skipped.InitiatingOUID),
			log.String("declaredPolicyID", skipped.DeclaredPolicyID))
	}
	if replayErr == nil {
		return nil
	}

	// A document names its policies only by position, so the refusal says which one.
	detail := tidcommon.CustomServiceError(*replayErr.Err, tidcommon.I18nMessage{
		Key: replayErr.Err.ErrorDescription.Key,
		DefaultValue: fmt.Sprintf("sharing policy %d (organization unit %s): %s",
			replayErr.Index+1, replayErr.InitiatingOUID, replayErr.Err.ErrorDescription.DefaultValue),
	})
	outcome := serviceErrorOutcome(resourceType, resourceID, resourceName, operation, detail)
	// The title alone says what kind of refusal it was; the description is what names the entry.
	outcome.Message = detail.ErrorDescription.DefaultValue
	return &outcome
}

func unsupportedAdapterOutcome(resourceType, name string) ImportItemOutcome {
	return ImportItemOutcome{
		ResourceType: resourceType,
		Status:       statusFailed,
		Code:         ErrorInvalidImportRequest.Code,
		Message:      name + " adapter is not configured",
	}
}

func decodeErrorOutcome(resourceType, id, name string, err error) ImportItemOutcome {
	return ImportItemOutcome{
		ResourceType: resourceType,
		ResourceID:   id,
		ResourceName: name,
		Status:       statusFailed,
		Code:         ErrorInvalidYAMLContent.Code,
		Message:      fmt.Sprintf("failed to decode %s document: %v", resourceType, err),
	}
}

func serviceErrorOutcome(
	resourceType, id, name, operation string,
	svcErr *tidcommon.ServiceError,
) ImportItemOutcome {
	return ImportItemOutcome{
		ResourceType: resourceType,
		ResourceID:   id,
		ResourceName: name,
		Operation:    operation,
		Status:       statusFailed,
		Code:         svcErr.Code,
		Message:      svcErr.Error.DefaultValue,
	}
}

func successOutcome(resourceType, id, name, operation string) ImportItemOutcome {
	return ImportItemOutcome{
		ResourceType: resourceType,
		ResourceID:   id,
		ResourceName: name,
		Operation:    operation,
		Status:       statusSuccess,
	}
}

//nolint:dupl // parallel to importCredentialConfiguration; kept separate per resource type.
func (s *importService) importPresentationDefinition(
	ctx context.Context, doc parsedDocument, options *ImportOptions, dryRun bool,
) ImportItemOutcome {
	if s.presentationDefinitionService == nil {
		return ImportItemOutcome{
			ResourceType: resourceTypePresentationDefinition,
			Status:       statusFailed,
			Code:         ErrorAdapterNotConfigured.Code,
			Message:      "presentation definition adapter is not configured",
		}
	}

	var dto presentation.PresentationDefinitionDTO
	if err := doc.Node.Decode(&dto); err != nil {
		return ImportItemOutcome{
			ResourceType: resourceTypePresentationDefinition,
			Status:       statusFailed,
			Code:         ErrorInvalidYAMLContent.Code,
			Message:      fmt.Sprintf("failed to decode presentation definition document: %v", err),
		}
	}

	resolvedOUID, svcErr := s.resolveImportOUHandle(
		ctx, resourceTypePresentationDefinition, dto.ID, dto.Handle, dto.OUID, dto.OUHandle)
	if svcErr != nil {
		return serviceErrorOutcome(
			resourceTypePresentationDefinition, dto.ID, dto.Handle, operationCreate, svcErr)
	}
	dto.OUID = resolvedOUID

	if dryRun {
		if options.IsUpsertEnabled() && dto.ID != "" {
			_, svcErr := s.presentationDefinitionService.GetPresentationDefinition(ctx, dto.ID)
			if svcErr == nil {
				return successOutcome(resourceTypePresentationDefinition, dto.ID, dto.Handle, operationUpdate)
			}
			if !isNotFoundServiceError(svcErr) {
				return serviceErrorOutcome(
					resourceTypePresentationDefinition, dto.ID, dto.Handle, operationUpdate, svcErr)
			}
		}
		return successOutcome(resourceTypePresentationDefinition, dto.ID, dto.Handle, operationCreate)
	}

	if options.IsUpsertEnabled() && dto.ID != "" {
		updated, svcErr := s.presentationDefinitionService.UpdatePresentationDefinition(ctx, dto.ID, &dto)
		if svcErr == nil {
			return successOutcome(resourceTypePresentationDefinition, updated.ID, updated.Handle, operationUpdate)
		}
		if !isNotFoundServiceError(svcErr) {
			return serviceErrorOutcome(
				resourceTypePresentationDefinition, dto.ID, dto.Handle, operationUpdate, svcErr)
		}
	}

	created, svcErr := s.presentationDefinitionService.CreatePresentationDefinition(ctx, &dto)
	if svcErr != nil {
		return serviceErrorOutcome(resourceTypePresentationDefinition, dto.ID, dto.Handle, operationCreate, svcErr)
	}
	return successOutcome(resourceTypePresentationDefinition, created.ID, created.Handle, operationCreate)
}

//nolint:dupl // parallel to importPresentationDefinition; kept separate per resource type.
func (s *importService) importCredentialConfiguration(
	ctx context.Context, doc parsedDocument, options *ImportOptions, dryRun bool,
) ImportItemOutcome {
	if s.credentialConfigurationService == nil {
		return ImportItemOutcome{
			ResourceType: resourceTypeCredentialConfiguration,
			Status:       statusFailed,
			Code:         ErrorAdapterNotConfigured.Code,
			Message:      "credential configuration adapter is not configured",
		}
	}

	var dto credential.CredentialConfigurationDTO
	if err := doc.Node.Decode(&dto); err != nil {
		return ImportItemOutcome{
			ResourceType: resourceTypeCredentialConfiguration,
			Status:       statusFailed,
			Code:         ErrorInvalidYAMLContent.Code,
			Message:      fmt.Sprintf("failed to decode credential configuration document: %v", err),
		}
	}

	resolvedOUID, svcErr := s.resolveImportOUHandle(
		ctx, resourceTypeCredentialConfiguration, dto.ID, dto.Handle, dto.OUID, dto.OUHandle)
	if svcErr != nil {
		return serviceErrorOutcome(resourceTypeCredentialConfiguration, dto.ID, dto.Handle, operationCreate, svcErr)
	}
	dto.OUID = resolvedOUID

	if dryRun {
		if options.IsUpsertEnabled() && dto.ID != "" {
			_, svcErr := s.credentialConfigurationService.GetCredentialConfiguration(ctx, dto.ID)
			if svcErr == nil {
				return successOutcome(resourceTypeCredentialConfiguration, dto.ID, dto.Handle, operationUpdate)
			}
			if !isNotFoundServiceError(svcErr) {
				return serviceErrorOutcome(
					resourceTypeCredentialConfiguration, dto.ID, dto.Handle, operationUpdate, svcErr)
			}
		}
		return successOutcome(resourceTypeCredentialConfiguration, dto.ID, dto.Handle, operationCreate)
	}

	if options.IsUpsertEnabled() && dto.ID != "" {
		updated, svcErr := s.credentialConfigurationService.UpdateCredentialConfiguration(ctx, dto.ID, &dto)
		if svcErr == nil {
			return successOutcome(resourceTypeCredentialConfiguration, updated.ID, updated.Handle, operationUpdate)
		}
		if !isNotFoundServiceError(svcErr) {
			return serviceErrorOutcome(
				resourceTypeCredentialConfiguration, dto.ID, dto.Handle, operationUpdate, svcErr)
		}
	}

	created, svcErr := s.credentialConfigurationService.CreateCredentialConfiguration(ctx, &dto)
	if svcErr != nil {
		return serviceErrorOutcome(resourceTypeCredentialConfiguration, dto.ID, dto.Handle, operationCreate, svcErr)
	}
	return successOutcome(resourceTypeCredentialConfiguration, created.ID, created.Handle, operationCreate)
}

// importNotificationTemplate imports a notification_template document.
// Templates are keyed by (channel, handle); with upsert, existing templates are matched by handle and updated.
func (s *importService) importNotificationTemplate(
	ctx context.Context, doc parsedDocument, options *ImportOptions, dryRun bool,
) ImportItemOutcome {
	if s.notifTemplateService == nil {
		return unsupportedAdapterOutcome(resourceTypeNotificationTemplate, "notification template")
	}

	var raw notificationTemplateDeclarativeYAML
	if err := doc.Node.Decode(&raw); err != nil {
		return decodeErrorOutcome(resourceTypeNotificationTemplate, "", raw.Handle, err)
	}

	resolvedOUID, svcErr := s.resolveImportOUHandle(
		ctx, resourceTypeNotificationTemplate, "", raw.Handle, raw.OUID, raw.OUHandle)
	if svcErr != nil {
		return serviceErrorOutcome(resourceTypeNotificationTemplate, "", raw.Handle, operationCreate, svcErr)
	}
	raw.OUID = resolvedOUID

	channel := notificationtemplate.ChannelType(raw.Channel)
	content := notificationtemplate.TemplateContent{Subject: raw.Content.Subject, Body: raw.Content.Body}
	var design *notificationtemplate.TemplateDesign
	if raw.Design != nil {
		design = &notificationtemplate.TemplateDesign{ColorScheme: raw.Design.ColorScheme}
	}
	createReq := notificationtemplate.CreateTemplateRequest{
		ID:          raw.ID,
		Handle:      raw.Handle,
		OUID:        raw.OUID,
		DisplayName: raw.DisplayName,
		Description: raw.Description,
		Design:      design,
		Content:     content,
	}

	// Validate before the handle lookup so an invalid or missing handle surfaces as a validation
	// failure, consistently between a dry run and the real apply, rather than being reported by the
	// handle-based lookup (which would otherwise mislabel it as an update).
	if svcErr := s.notifTemplateService.ValidateTemplate(ctx, channel, createReq); svcErr != nil {
		return serviceErrorOutcome(resourceTypeNotificationTemplate, "", raw.Handle, operationCreate, svcErr)
	}

	// Resolve whether the handle already exists so the dry run can predict the real apply's outcome and
	// upsert can target the existing template. For a real non-upsert apply the create path surfaces any
	// conflict, so the lookup is only needed for a dry run or an upsert.
	existingID := ""
	existingOUID := ""
	if dryRun || options.IsUpsertEnabled() {
		existing, lookupErr := s.notifTemplateService.GetTemplateByHandle(ctx, channel, raw.Handle)
		if lookupErr != nil && !isNotFoundServiceError(lookupErr) {
			return serviceErrorOutcome(resourceTypeNotificationTemplate, "", raw.Handle, operationUpdate, lookupErr)
		}
		if existing != nil {
			existingID = existing.ID
			existingOUID = existing.OUID
		}
	}

	// The ouId is immutable; an update that moves an existing template to a different OU is rejected
	// rather than silently ignored (UpdateTemplate preserves the stored OU). Checked for both the dry
	// run and the real apply so the prediction matches.
	ouMismatch := func() bool { return existingID != "" && raw.OUID != "" && raw.OUID != existingOUID }

	if dryRun {
		op := operationCreate
		if existingID != "" && options.IsUpsertEnabled() {
			op = operationUpdate
		}
		// Without upsert, an existing (channel, handle) is a conflict the real apply would hit.
		if existingID != "" && !options.IsUpsertEnabled() {
			return serviceErrorOutcome(resourceTypeNotificationTemplate, existingID, raw.Handle,
				operationCreate, &notificationtemplate.ErrorTemplateHandleConflict)
		}
		if op == operationUpdate && ouMismatch() {
			return serviceErrorOutcome(resourceTypeNotificationTemplate, existingID, raw.Handle,
				operationUpdate, &notificationtemplate.ErrorOUImmutable)
		}
		return successOutcome(resourceTypeNotificationTemplate, existingID, raw.Handle, op)
	}

	if existingID == "" {
		created, svcErr := s.notifTemplateService.CreateTemplate(ctx, channel, createReq)
		if svcErr == nil {
			return successOutcome(resourceTypeNotificationTemplate, created.ID, created.Handle, operationCreate)
		}
		// Create race: the handle appeared between the existence check and the insert. Fall back to
		// updating it when upsert is on; otherwise surface the conflict as a create failure.
		if svcErr.Code != notificationtemplate.ErrorTemplateHandleConflict.Code || !options.IsUpsertEnabled() {
			return serviceErrorOutcome(resourceTypeNotificationTemplate, "", raw.Handle, operationCreate, svcErr)
		}
		existing, lookupErr := s.notifTemplateService.GetTemplateByHandle(ctx, channel, raw.Handle)
		if lookupErr != nil && !isNotFoundServiceError(lookupErr) {
			return serviceErrorOutcome(resourceTypeNotificationTemplate, "", raw.Handle, operationUpdate, lookupErr)
		}
		if existing == nil {
			return serviceErrorOutcome(resourceTypeNotificationTemplate, "", raw.Handle, operationCreate, svcErr)
		}
		existingID = existing.ID
		existingOUID = existing.OUID
	}

	// Reject an attempt to move an existing template to a different OU (ouId is immutable).
	if ouMismatch() {
		return serviceErrorOutcome(resourceTypeNotificationTemplate, existingID, raw.Handle,
			operationUpdate, &notificationtemplate.ErrorOUImmutable)
	}

	updated, updErr := s.notifTemplateService.UpdateTemplate(ctx, channel, existingID,
		notificationtemplate.UpdateTemplateRequest{
			DisplayName: raw.DisplayName,
			Description: raw.Description,
			Design:      design,
			Content:     content,
		})
	if updErr != nil {
		return serviceErrorOutcome(resourceTypeNotificationTemplate, existingID, raw.Handle, operationUpdate, updErr)
	}
	return successOutcome(resourceTypeNotificationTemplate, updated.ID, updated.Handle, operationUpdate)
}

// importAuthorizationDetailType imports an authorization_detail_type document.
func (s *importService) importAuthorizationDetailType(
	ctx context.Context, doc parsedDocument, options *ImportOptions, dryRun bool,
) ImportItemOutcome {
	if s.authorizationDetailTypeService == nil {
		return unsupportedAdapterOutcome(resourceTypeAuthorizationDetailType, "authorization details type")
	}

	id, name := extractDocumentIdentity(doc)
	t, err := authzdetailmgt.ParseAuthorizationDetailTypeNode(doc.Node)
	if err != nil {
		return decodeErrorOutcome(resourceTypeAuthorizationDetailType, id, name, err)
	}

	if dryRun {
		return s.dryRunAuthorizationDetailType(ctx, t, options)
	}

	if options.IsUpsertEnabled() && t.ID != "" {
		updated, svcErr := s.authorizationDetailTypeService.UpdateAuthorizationDetailType(ctx, t.ID, *t)
		if svcErr == nil {
			return successOutcome(resourceTypeAuthorizationDetailType, updated.ID, updated.Name, operationUpdate)
		}

		if !isNotFoundServiceError(svcErr) {
			return serviceErrorOutcome(resourceTypeAuthorizationDetailType, t.ID, t.Name, operationUpdate, svcErr)
		}
	}

	created, svcErr := s.authorizationDetailTypeService.CreateAuthorizationDetailType(ctx, *t)
	if svcErr != nil {
		return serviceErrorOutcome(resourceTypeAuthorizationDetailType, t.ID, t.Name, operationCreate, svcErr)
	}
	return successOutcome(resourceTypeAuthorizationDetailType, created.ID, created.Name, operationCreate)
}

// dryRunAuthorizationDetailType predicts the outcome of importing a type, refusing it in the order the
// real update or create would.
func (s *importService) dryRunAuthorizationDetailType(ctx context.Context, t *providers.AuthorizationDetailType,
	options *ImportOptions) ImportItemOutcome {
	var existing *providers.AuthorizationDetailType
	if t.ID != "" {
		found, svcErr := s.authorizationDetailTypeService.GetAuthorizationDetailType(ctx, t.ID)
		if svcErr != nil && !isNotFoundServiceError(svcErr) {
			return serviceErrorOutcome(resourceTypeAuthorizationDetailType, t.ID, t.Name, operationUpdate, svcErr)
		}
		existing = found
	}
	if existing != nil && options.IsUpsertEnabled() {
		if existing.IsReadOnly {
			return serviceErrorOutcome(resourceTypeAuthorizationDetailType, t.ID, t.Name, operationUpdate,
				&authzdetailmgt.ErrorImmutableType)
		}
		if existing.Type != t.Type {
			return serviceErrorOutcome(resourceTypeAuthorizationDetailType, t.ID, t.Name, operationUpdate,
				&authzdetailmgt.ErrorTypeValueImmutable)
		}
		if svcErr := s.authorizationDetailTypeService.ValidateAuthorizationDetailType(ctx, *t); svcErr != nil {
			return serviceErrorOutcome(resourceTypeAuthorizationDetailType, t.ID, t.Name, operationUpdate, svcErr)
		}
		return successOutcome(resourceTypeAuthorizationDetailType, t.ID, t.Name, operationUpdate)
	}

	if authzdetailmgt.IsDeclarativeModeEnabled() {
		return serviceErrorOutcome(resourceTypeAuthorizationDetailType, t.ID, t.Name, operationCreate,
			&authzdetailmgt.ErrorDeclarativeModeCreateNotAllowed)
	}
	// A create takes the credential as sent, so it is not checked against a stored type that holds the id.
	candidate := *t
	candidate.ID = ""
	if svcErr := s.authorizationDetailTypeService.ValidateAuthorizationDetailType(ctx, candidate); svcErr != nil {
		return serviceErrorOutcome(resourceTypeAuthorizationDetailType, t.ID, t.Name, operationCreate, svcErr)
	}
	byValue, svcErr := s.authorizationDetailTypeService.GetAuthorizationDetailTypeByValue(ctx, t.Type)
	if svcErr != nil {
		return serviceErrorOutcome(resourceTypeAuthorizationDetailType, t.ID, t.Name, operationCreate, svcErr)
	}
	if byValue != nil {
		return serviceErrorOutcome(resourceTypeAuthorizationDetailType, t.ID, t.Name, operationCreate,
			&authzdetailmgt.ErrorTypeConflict)
	}
	if existing != nil {
		return serviceErrorOutcome(resourceTypeAuthorizationDetailType, t.ID, t.Name, operationCreate,
			&authzdetailmgt.ErrorTypeIDConflict)
	}
	return successOutcome(resourceTypeAuthorizationDetailType, t.ID, t.Name, operationCreate)
}
