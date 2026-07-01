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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package importer

import (
	"context"
	"encoding/json"
	"fmt"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	agentmodel "github.com/thunder-id/thunderid/internal/agent/model"
	layoutmgt "github.com/thunder-id/thunderid/internal/design/layout/mgt"
	thememgt "github.com/thunder-id/thunderid/internal/design/theme/mgt"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/openid4vci/credential"
	"github.com/thunder-id/thunderid/internal/openid4vp/definition"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/role"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/user"
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

type roleDeclarativeYAML struct {
	ID          string                     `yaml:"id"`
	Name        string                     `yaml:"name"`
	Description string                     `yaml:"description,omitempty"`
	OUID        string                     `yaml:"ouId,omitempty"`
	OUHandle    string                     `yaml:"ouHandle,omitempty"`
	Permissions []role.ResourcePermissions `yaml:"permissions"`
	Assignments []role.RoleAssignment      `yaml:"assignments,omitempty"`
}

type userDeclarativeYAML struct {
	ID          string                 `yaml:"id"`
	Type        string                 `yaml:"type"`
	OUID        string                 `yaml:"ouId,omitempty"`
	OUHandle    string                 `yaml:"ouHandle,omitempty"`
	Attributes  map[string]interface{} `yaml:"attributes"`
	Credentials map[string]interface{} `yaml:"credentials,omitempty"`
}

type entityTypeDeclarativeYAML struct {
	ID                    string                       `yaml:"id"`
	Category              entitytype.TypeCategory      `yaml:"category,omitempty"`
	Name                  string                       `yaml:"name"`
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

func (s *importService) importOrganizationUnit(
	ctx context.Context, doc parsedDocument, options *ImportOptions, dryRun bool,
) ImportItemOutcome {
	if s.ouService == nil {
		return unsupportedAdapterOutcome(resourceTypeOrganizationUnit, "organization unit")
	}

	var req providers.OrganizationUnit
	if err := doc.Node.Decode(&req); err != nil {
		return decodeErrorOutcome(resourceTypeOrganizationUnit, req.ID, req.Name, err)
	}

	createReq := providers.OrganizationUnitRequestWithID{
		ID:              req.ID,
		Handle:          req.Handle,
		Name:            req.Name,
		Description:     req.Description,
		Parent:          req.Parent,
		ThemeID:         req.ThemeID,
		LayoutID:        req.LayoutID,
		LogoURL:         req.LogoURL,
		TosURI:          req.TosURI,
		PolicyURI:       req.PolicyURI,
		CookiePolicyURI: req.CookiePolicyURI,
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
		return decodeErrorOutcome(resourceTypeEntityType, req.ID, req.Name, err)
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
				ResourceName: req.Name,
				Status:       statusFailed,
				Code:         ErrorInvalidYAMLContent.Code,
				Message:      fmt.Sprintf("failed to marshal schema: %v", err),
			}
		}
	}

	category := req.Category
	if category == "" {
		category = entitytype.TypeCategoryUser
	}
	if !category.IsValid() {
		return ImportItemOutcome{
			ResourceType: resourceTypeEntityType,
			ResourceID:   req.ID,
			ResourceName: req.Name,
			Status:       statusFailed,
			Code:         ErrorInvalidYAMLContent.Code,
			Message:      fmt.Sprintf("invalid entity type category %q", string(category)),
		}
	}

	createReq := entitytype.CreateEntityTypeRequestWithID{
		ID:                    req.ID,
		Name:                  req.Name,
		OUID:                  req.OUID,
		OUHandle:              req.OUHandle,
		AllowSelfRegistration: req.AllowSelfRegistration,
		SystemAttributes:      req.SystemAttributes,
		Schema:                schemaBytes,
	}
	updateReq := entitytype.UpdateEntityTypeRequest{
		Name:                  createReq.Name,
		OUID:                  createReq.OUID,
		OUHandle:              createReq.OUHandle,
		AllowSelfRegistration: createReq.AllowSelfRegistration,
		SystemAttributes:      createReq.SystemAttributes,
		Schema:                createReq.Schema,
	}

	if dryRun {
		if options.IsUpsertEnabled() && req.ID != "" {
			_, svcErr := s.entityTypeService.GetEntityType(ctx, category, req.ID, false)
			if svcErr == nil {
				return successOutcome(resourceTypeEntityType, req.ID, req.Name, operationUpdate)
			}

			if !isNotFoundServiceError(svcErr) {
				return serviceErrorOutcome(resourceTypeEntityType, req.ID, req.Name, operationUpdate, svcErr)
			}
		}

		return successOutcome(resourceTypeEntityType, req.ID, req.Name, operationCreate)
	}

	if options.IsUpsertEnabled() && req.ID != "" {
		updated, svcErr := s.entityTypeService.UpdateEntityType(ctx, category, req.ID, updateReq)
		if svcErr == nil {
			return successOutcome(resourceTypeEntityType, updated.ID, updated.Name, operationUpdate)
		}

		if !isNotFoundServiceError(svcErr) {
			return serviceErrorOutcome(resourceTypeEntityType, req.ID, req.Name, operationUpdate, svcErr)
		}

		created, createErr := s.entityTypeService.CreateEntityType(ctx, category, createReq)
		if createErr != nil {
			return serviceErrorOutcome(resourceTypeEntityType, req.ID, req.Name, operationCreate, createErr)
		}
		return successOutcome(resourceTypeEntityType, created.ID, created.Name, operationCreate)
	}

	created, svcErr := s.entityTypeService.CreateEntityType(ctx, category, createReq)
	if svcErr != nil {
		return serviceErrorOutcome(resourceTypeEntityType, req.ID, req.Name, operationCreate, svcErr)
	}
	return successOutcome(resourceTypeEntityType, created.ID, created.Name, operationCreate)
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

	createReq := role.RoleCreationDetail{
		ID:          req.ID,
		Name:        req.Name,
		Description: req.Description,
		OUID:        req.OUID,
		Permissions: req.Permissions,
		Assignments: req.Assignments,
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
			if len(req.Assignments) > 0 {
				if s.roleAssignmentService == nil {
					return serviceErrorOutcome(resourceTypeRole, updated.ID, updated.Name, operationUpdate,
						tidcommon.CustomServiceError(tidcommon.InternalServerError,
							tidcommon.I18nMessage{DefaultValue: "roleAssignmentService not configured"}))
				}
				assignErr := s.roleAssignmentService.AddAssignments(ctx, updated.ID, req.Assignments)
				if assignErr != nil {
					return serviceErrorOutcome(resourceTypeRole, updated.ID, updated.Name, operationUpdate, assignErr)
				}
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
	return successOutcome(resourceTypeRole, created.ID, created.Name, operationCreate)
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

	if dryRun {
		if options.IsUpsertEnabled() && req.ID != "" {
			_, svcErr := s.themeService.GetTheme(ctx, req.ID)
			if svcErr == nil {
				return successOutcome(resourceTypeTheme, req.ID, req.DisplayName, operationUpdate)
			}

			if !isNotFoundServiceError(svcErr) {
				return serviceErrorOutcome(resourceTypeTheme, req.ID, req.DisplayName, operationUpdate, svcErr)
			}
		}

		return successOutcome(resourceTypeTheme, req.ID, req.DisplayName, operationCreate)
	}

	if options.IsUpsertEnabled() && req.ID != "" {
		updated, svcErr := s.themeService.UpdateTheme(ctx, req.ID, updateReq)
		if svcErr == nil {
			return successOutcome(resourceTypeTheme, updated.ID, updated.DisplayName, operationUpdate)
		}

		if !isNotFoundServiceError(svcErr) {
			return serviceErrorOutcome(resourceTypeTheme, req.ID, req.DisplayName, operationUpdate, svcErr)
		}

		created, createErr := s.themeService.CreateTheme(ctx, createReq)
		if createErr != nil {
			return serviceErrorOutcome(resourceTypeTheme, req.ID, req.DisplayName, operationCreate, createErr)
		}

		return successOutcome(resourceTypeTheme, created.ID, created.DisplayName, operationCreate)
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

	createReq := layoutmgt.CreateLayoutRequest{
		Handle:      req.Handle,
		DisplayName: req.DisplayName,
		Description: req.Description,
		Layout:      layoutBytes,
	}
	updateReq := layoutmgt.UpdateLayoutRequest(createReq)

	return importDesignResource(options.IsUpsertEnabled(), dryRun, req.ID, req.DisplayName,
		func() *tidcommon.ServiceError {
			_, svcErr := s.layoutService.GetLayout(ctx, req.ID)
			return svcErr
		},
		func() (string, string, *tidcommon.ServiceError) {
			updated, svcErr := s.layoutService.UpdateLayout(ctx, req.ID, updateReq)
			if svcErr != nil {
				return "", "", svcErr
			}
			return updated.ID, updated.DisplayName, nil
		},
		func() (string, string, *tidcommon.ServiceError) {
			created, svcErr := s.layoutService.CreateLayout(ctx, createReq)
			if svcErr != nil {
				return "", "", svcErr
			}
			return created.ID, created.DisplayName, nil
		},
		resourceTypeLayout,
	)
}

func (s *importService) importUser(
	ctx context.Context, doc parsedDocument, options *ImportOptions, dryRun bool,
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

	userReq := &user.User{
		ID:         req.ID,
		OUID:       req.OUID,
		Type:       req.Type,
		Attributes: attributesJSON,
	}

	credentialsJSON, err := json.Marshal(req.Credentials)
	if err != nil {
		return ImportItemOutcome{ResourceType: resourceTypeUser, ResourceID: req.ID, Status: statusFailed,
			Code: ErrorInvalidYAMLContent.Code, Message: fmt.Sprintf("failed to marshal user credentials: %v", err)}
	}

	if dryRun {
		if options.IsUpsertEnabled() && req.ID != "" {
			_, svcErr := s.userService.GetUser(ctx, req.ID, false)
			if svcErr == nil {
				return successOutcome(resourceTypeUser, req.ID, "", operationUpdate)
			}

			if !isNotFoundServiceError(svcErr) {
				return serviceErrorOutcome(resourceTypeUser, req.ID, "", operationUpdate, svcErr)
			}
		}

		return successOutcome(resourceTypeUser, req.ID, "", operationCreate)
	}

	if options.IsUpsertEnabled() && req.ID != "" {
		updated, svcErr := s.userService.UpdateUser(ctx, req.ID, userReq)
		if svcErr == nil {
			if len(credentialsJSON) > 0 && string(credentialsJSON) != "null" && string(credentialsJSON) != "{}" {
				if credErr := s.userService.UpdateUserCredentials(
					ctx,
					req.ID,
					json.RawMessage(credentialsJSON),
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
			return successOutcome(resourceTypeUser, updated.ID, "", operationUpdate)
		}

		if !isNotFoundServiceError(svcErr) {
			return serviceErrorOutcome(resourceTypeUser, req.ID, "", operationUpdate, svcErr)
		}
	}

	created, svcErr := s.userService.CreateUser(ctx, userReq)
	if svcErr != nil {
		return serviceErrorOutcome(resourceTypeUser, req.ID, "", operationCreate, svcErr)
	}
	if len(credentialsJSON) > 0 && string(credentialsJSON) != "null" && string(credentialsJSON) != "{}" {
		if credErr := s.userService.UpdateUserCredentials(
			ctx,
			created.ID,
			json.RawMessage(credentialsJSON),
		); credErr != nil {
			if rollbackErr := s.userService.DeleteUser(ctx, created.ID); rollbackErr != nil {
				combinedErr := &tidcommon.ServiceError{
					Code: credErr.Code,
					Type: credErr.Type,
					Error: tidcommon.I18nMessage{
						Key: credErr.Error.Key,
						DefaultValue: fmt.Sprintf(
							"user credential update failed: %s; rollback delete failed: %s",
							credErr.Error.DefaultValue,
							rollbackErr.Error.DefaultValue,
						),
					},
					ErrorDescription: tidcommon.I18nMessage{
						Key: credErr.ErrorDescription.Key,
						DefaultValue: fmt.Sprintf(
							"credential update error code %s for user %s; rollback delete error code %s",
							credErr.Code,
							created.ID,
							rollbackErr.Code,
						),
					},
				}

				return serviceErrorOutcome(resourceTypeUser, created.ID, "", operationCreate, combinedErr)
			}

			return serviceErrorOutcome(resourceTypeUser, created.ID, "", operationCreate, credErr)
		}
	}

	return successOutcome(resourceTypeUser, created.ID, "", operationCreate)
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

// importResourceServerChildren creates resources and actions nested under a resource server.
// It first computes permission strings via ProcessResourceServer, then calls the resource service
// for each resource and action.  Existing resources/actions (on upsert paths) are silently skipped.
func (s *importService) importResourceServerChildren(
	ctx context.Context, serverID string, rs providers.ResourceServer,
) *tidcommon.ServiceError {
	if len(rs.Resources) == 0 {
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

	// handleToID maps resource handle → created/resolved ID for parent resolution.
	handleToID := make(map[string]string)

	for i := range rs.Resources {
		res := rs.Resources[i]

		// Resolve ParentHandle to the parent ID using handles seen so far in this import.
		if res.ParentHandle != "" {
			if parentID, ok := handleToID[res.ParentHandle]; ok {
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
				if pid, ok := handleToID[res.ParentHandle]; ok {
					parentID = &pid
				}
			}
			list, listErr := s.resourceService.GetResourceList(ctx, serverID, parentID, serverconst.MaxPageSize, 0)
			if listErr != nil {
				return listErr
			}
			var existingID string
			for j := range list.Resources {
				if list.Resources[j].Handle == res.Handle {
					existingID = list.Resources[j].ID
					break
				}
			}
			if existingID == "" {
				continue
			}
			created = &providers.Resource{ID: existingID}
		}

		handleToID[res.Handle] = created.ID

		for j := range res.Actions {
			action := res.Actions[j]
			_, actionErr := s.resourceService.CreateAction(ctx, serverID, &created.ID, action)
			if actionErr != nil && actionErr.Code != resource.ErrorHandleConflict.Code {
				return actionErr
			}
		}
	}

	return nil
}

func (s *importService) importAgent(
	ctx context.Context, doc parsedDocument, options *ImportOptions, dryRun bool, flowIDAliases map[string]string,
) ImportItemOutcome {
	if s.agentService == nil {
		return unsupportedAdapterOutcome(resourceTypeAgent, "agent")
	}

	var req agentmodel.AgentRequestWithID
	if err := doc.Node.Decode(&req); err != nil {
		return decodeErrorOutcome(resourceTypeAgent, req.ID, req.Name, err)
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
				ResourceName: req.Name,
				Status:       statusFailed,
				Code:         ErrorInvalidYAMLContent.Code,
				Message:      fmt.Sprintf("failed to marshal agent attributes: %v", err),
			}
		}
		attributesJSON = raw
	}

	normalizeAgentOAuthConfigForImport(ctx, &req)

	createReq := &agentmodel.Agent{
		ID:                 req.ID,
		OUID:               req.OUID,
		OUHandle:           req.OUHandle,
		Type:               req.Type,
		Name:               req.Name,
		Description:        req.Description,
		Owner:              req.Owner,
		Attributes:         attributesJSON,
		InboundAuthProfile: req.InboundAuthProfile,
		InboundAuthConfig:  req.InboundAuthConfig,
	}
	updateReq := &agentmodel.UpdateAgentRequest{
		OUID:               req.OUID,
		OUHandle:           req.OUHandle,
		Type:               req.Type,
		Name:               req.Name,
		Description:        req.Description,
		Owner:              req.Owner,
		Attributes:         attributesJSON,
		InboundAuthProfile: req.InboundAuthProfile,
		InboundAuthConfig:  req.InboundAuthConfig,
	}

	if dryRun {
		if options.IsUpsertEnabled() && req.ID != "" {
			_, svcErr := s.agentService.GetAgent(ctx, req.ID, false)
			if svcErr == nil {
				return successOutcome(resourceTypeAgent, req.ID, req.Name, operationUpdate)
			}

			if !isNotFoundServiceError(svcErr) {
				return serviceErrorOutcome(resourceTypeAgent, req.ID, req.Name, operationUpdate, svcErr)
			}
		}

		return successOutcome(resourceTypeAgent, req.ID, req.Name, operationCreate)
	}

	if options.IsUpsertEnabled() && req.ID != "" {
		_, svcErr := s.agentService.GetAgent(ctx, req.ID, false)
		if svcErr == nil {
			updated, updateErr := s.agentService.UpdateAgent(ctx, req.ID, updateReq)
			if updateErr != nil {
				return serviceErrorOutcome(resourceTypeAgent, req.ID, req.Name, operationUpdate, updateErr)
			}
			return successOutcome(resourceTypeAgent, updated.ID, updated.Name, operationUpdate)
		}

		if !isNotFoundServiceError(svcErr) {
			return serviceErrorOutcome(resourceTypeAgent, req.ID, req.Name, operationUpdate, svcErr)
		}
	}

	created, svcErr := s.agentService.CreateAgent(ctx, createReq)
	if svcErr != nil {
		return serviceErrorOutcome(resourceTypeAgent, req.ID, req.Name, operationCreate, svcErr)
	}
	return successOutcome(resourceTypeAgent, created.ID, created.Name, operationCreate)
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

	if oauthConfig.PublicClient &&
		oauthConfig.TokenEndpointAuthMethod == providers.TokenEndpointAuthMethodNone &&
		oauthConfig.ClientSecret != "" {
		log.GetLogger().Debug(ctx,
			"Dropping client_secret for public agent import with token endpoint auth method 'none'",
			log.String("agentID", req.ID),
			log.String("name", req.Name),
			log.String("clientID", oauthConfig.ClientID))
		oauthConfig.ClientSecret = ""
	}
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

func importDesignResource(
	upsert bool,
	dryRun bool,
	resourceID string,
	resourceName string,
	getFn func() *tidcommon.ServiceError,
	updateFn func() (string, string, *tidcommon.ServiceError),
	createFn func() (string, string, *tidcommon.ServiceError),
	resourceType string,
) ImportItemOutcome {
	if dryRun {
		if upsert && resourceID != "" {
			svcErr := getFn()
			if svcErr == nil {
				return successOutcome(resourceType, resourceID, resourceName, operationUpdate)
			}

			if !isNotFoundServiceError(svcErr) {
				return serviceErrorOutcome(
					resourceType,
					resourceID,
					resourceName,
					operationUpdate,
					svcErr,
				)
			}
		}

		return successOutcome(resourceType, resourceID, resourceName, operationCreate)
	}

	if upsert && resourceID != "" {
		updatedID, updatedName, svcErr := updateFn()
		if svcErr == nil {
			return successOutcome(resourceType, updatedID, updatedName, operationUpdate)
		}

		if !isNotFoundServiceError(svcErr) {
			return serviceErrorOutcome(
				resourceType,
				resourceID,
				resourceName,
				operationUpdate,
				svcErr,
			)
		}

		// ID-preserving create is not supported; return a clear failure when ID is set but not found.
		return ImportItemOutcome{
			ResourceType: resourceType,
			ResourceID:   resourceID,
			ResourceName: resourceName,
			Operation:    operationCreate,
			Status:       statusFailed,
			Code:         ErrorInvalidImportRequest.Code,
			Message: fmt.Sprintf("%s with given ID not found; ID-preserving create not supported",
				resourceType),
		}
	}

	createdID, createdName, svcErr := createFn()
	if svcErr != nil {
		return serviceErrorOutcome(resourceType, resourceID, resourceName, operationCreate, svcErr)
	}

	return successOutcome(resourceType, createdID, createdName, operationCreate)
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

	var dto definition.PresentationDefinitionDTO
	if err := doc.Node.Decode(&dto); err != nil {
		return ImportItemOutcome{
			ResourceType: resourceTypePresentationDefinition,
			Status:       statusFailed,
			Code:         ErrorInvalidYAMLContent.Code,
			Message:      fmt.Sprintf("failed to decode presentation definition document: %v", err),
		}
	}

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
