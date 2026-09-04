// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package discovery implements the SCIM discovery endpoints (ServiceProviderConfig,
// Schemas, ResourceTypes) per RFC 7643/7644.
package discovery

import (
	"context"
	"fmt"
	"strings"

	"github.com/thunder-id/thunderid/internal/entitytype"
	scim "github.com/thunder-id/thunderid/internal/scim/common"
	scimconfig "github.com/thunder-id/thunderid/internal/scim/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// SCIMDiscoveryServiceInterface defines the SCIM discovery service operations.
type SCIMDiscoveryServiceInterface interface {
	GetServiceProviderConfig(ctx context.Context, baseURL string) SCIMServiceProviderConfig
	ListSchemas(
		ctx context.Context, baseURL string,
	) (SCIMSchemaListResponse, *tidcommon.ServiceError)
	GetSchema(
		ctx context.Context, schemaURN string, baseURL string,
	) (*SCIMSchema, *tidcommon.ServiceError)
	ListResourceTypes(
		ctx context.Context, baseURL string,
	) (SCIMResourceTypeListResponse, *tidcommon.ServiceError)
	GetResourceType(
		ctx context.Context, resourceTypeID string, baseURL string,
	) (*SCIMResourceType, *tidcommon.ServiceError)
}

const discoveryServiceLoggerComponentName = "SCIMDiscoveryService"

// scimDiscoveryService coordinates SCIM discovery operations (ServiceProviderConfig,
// Schemas, ResourceTypes), delegating user type operations to existing ThunderID services.
type scimDiscoveryService struct {
	userTypeService entitytype.EntityTypeServiceInterface
	cfg             scimconfig.SCIMConfig
	serverStartTime string
	logger          log.Logger
}

// newSCIMDiscoveryService creates a new scimDiscoveryService instance.
func newSCIMDiscoveryService(
	userTypeService entitytype.EntityTypeServiceInterface,
	cfg scimconfig.SCIMConfig,
	serverStartTime string,
) *scimDiscoveryService {
	return &scimDiscoveryService{
		userTypeService: userTypeService,
		cfg:             cfg,
		serverStartTime: serverStartTime,
		logger:          *log.GetLogger().With(log.String(log.LoggerKeyComponentName, discoveryServiceLoggerComponentName)),
	}
}

// GetServiceProviderConfig returns the SCIM ServiceProviderConfig resource
// describing the server's supported capabilities per RFC 7643 §5.
func (s *scimDiscoveryService) GetServiceProviderConfig(_ context.Context, baseURL string) SCIMServiceProviderConfig {
	location := fmt.Sprintf("%s%s/ServiceProviderConfig", baseURL, scim.SCIMBasePath)

	meta := scim.SCIMMeta{
		ResourceType: "ServiceProviderConfig",
		Created:      s.serverStartTime,
		LastModified: s.serverStartTime, // equals Created — resource never modified by users
		Location:     location,
	}

	return SCIMServiceProviderConfig{
		Schemas: []string{scimServiceProviderConfigSchemaURN},
		Patch:   scimSupportedFeature{Supported: scimconfig.PatchSupported},
		Bulk: scimBulkConfig{
			Supported:      scimconfig.BulkSupported,
			MaxOperations:  scimconfig.BulkMaxOperations,
			MaxPayloadSize: scimconfig.BulkMaxPayloadSize,
		},
		Filter: scimFilterConfig{
			Supported:  scimconfig.FilterSupported,
			MaxResults: scimconfig.FilterMaxResults,
		},
		ChangePassword: scimSupportedFeature{Supported: scimconfig.ChangePasswordSupported},
		Sort:           scimSupportedFeature{Supported: scimconfig.SortSupported},
		ETag:           scimSupportedFeature{Supported: scimconfig.ETagSupported},
		Pagination: scimPaginationConfig{
			Cursor:                  scimconfig.PaginationCursorSupported,
			Index:                   scimconfig.PaginationIndexSupported,
			DefaultPaginationMethod: scimconfig.PaginationDefaultMethod,
			DefaultPageSize:         scimconfig.PaginationDefaultPageSize,
			MaxPageSize:             scimconfig.PaginationMaxPageSize,
		},
		AuthenticationSchemes: []scimAuthenticationScheme{
			{
				Type:        "oauthbearertoken",
				Name:        "OAuth Bearer Token",
				Description: "Authentication using an OAuth 2.0 Bearer Token",
			},
		},
		Meta: meta,
	}
}

// ListSchemas returns all SCIM schemas: the core User schema, the core Group schema,
// plus one extension schema per registered ThunderID user-type entity type.
func (s *scimDiscoveryService) ListSchemas(
	ctx context.Context, baseURL string,
) (SCIMSchemaListResponse, *tidcommon.ServiceError) {
	// --- 1. Collect all user type names (single shared paginator) ---
	names, svcErr := s.listUserTypeNames(ctx)
	if svcErr != nil {
		return SCIMSchemaListResponse{}, svcErr
	}

	// --- 2. Core User schema is included only if a core user type is resolvable and
	// loadable, same best-effort treatment as the per-user-type extension schemas below.
	// Core Group schema has no ThunderID user-type backing and is always included. ---
	schemas := make([]SCIMSchema, 0, 2+len(names))
	if coreType, svcErr := s.coreUserTypeFromNames(ctx, names); svcErr != nil {
		s.logger.Debug(ctx, "Core user type unavailable, omitting SCIM core User schema",
			log.Any("error", svcErr))
	} else if coreSchema, err := buildCoreUserSchema(baseURL, *coreType); err != nil {
		s.logger.Warn(ctx, "Failed to build SCIM core User schema, omitting", log.Error(err))
	} else {
		schemas = append(schemas, coreSchema)
	}
	schemas = append(schemas, buildCoreGroupSchema(baseURL))

	// --- 3. One extension schema per user type ---
	runtimeCtx := security.WithRuntimeContext(ctx)
	for _, name := range names {
		et, svcErr := s.userTypeService.GetEntityTypeByName(
			runtimeCtx, entitytype.TypeCategoryUser, name,
		)
		if svcErr != nil {
			s.logger.Warn(ctx, "Failed to load user type for SCIM schema list, skipping",
				log.String("userTypeName", name),
				log.Any("error", svcErr),
			)
			continue
		}

		scimSchema, err := mapUserTypeToSCIMSchema(*et, baseURL)
		if err != nil {
			s.logger.Warn(ctx, "Failed to map user type to SCIM schema, skipping",
				log.String("userTypeName", name),
				log.Error(err),
			)
			continue
		}
		schemas = append(schemas, scimSchema)
	}

	return SCIMSchemaListResponse{
		Schemas:      []string{scim.SCIMListResponseSchemaURN},
		TotalResults: len(schemas),
		StartIndex:   1,
		ItemsPerPage: len(schemas),
		Resources:    schemas,
	}, nil
}

// GetSchema returns a single SCIM Schema resource by URN. It returns the static
// core User or Group schema for their RFC 7643 URNs, or a dynamically built
// extension schema for a registered ThunderID user-type URN. Returns
// ErrorSchemaNotFound if the URN does not match any known schema.
func (s *scimDiscoveryService) GetSchema(
	ctx context.Context, schemaURN string, baseURL string,
) (*SCIMSchema, *tidcommon.ServiceError) {
	trimmedURN := strings.TrimSpace(schemaURN)
	if trimmedURN == "" {
		return nil, &scim.ErrorSchemaNotFound
	}

	// Case-insensitive URN comparison per RFC 7643 §1.2 which states schema URNs
	// "SHOULD" be compared case-insensitively.
	// --- 1. Core User schema (derived from the designated core user type, RFC 7643 §4.1) ---
	if strings.EqualFold(trimmedURN, scim.SCIMCoreUserSchemaURN) {
		coreType, svcErr := s.resolveCoreUserEntityType(ctx)
		if svcErr != nil {
			if svcErr.Type == tidcommon.ServerErrorType {
				return nil, &scim.ErrorInternalServer
			}
			// No core user type resolvable, or it failed to load → schema not found,
			// same treatment as the ThunderID extension schema branch below.
			s.logger.Debug(ctx, "Core user type unavailable for SCIM core User schema URN",
				log.Any("error", svcErr))
			return nil, &scim.ErrorSchemaNotFound
		}
		schema, err := buildCoreUserSchema(baseURL, *coreType)
		if err != nil {
			s.logger.Error(ctx, "Failed to build SCIM core User schema", log.Error(err))
			return nil, &scim.ErrorInternalServer
		}
		schema.Schemas = []string{scimSchemaSchemaURN}
		return &schema, nil
	}

	// --- 2. Core Group schema (static, RFC 7643 §4.2) ---
	if strings.EqualFold(trimmedURN, scim.SCIMCoreGroupSchemaURN) {
		schema := buildCoreGroupSchema(baseURL)
		schema.Schemas = []string{scimSchemaSchemaURN}
		return &schema, nil
	}

	// --- 3. ThunderID extension schema (dynamic, from DB) ---
	userTypeName, ok := scim.ParseUserTypeFromSchemaURN(trimmedURN)
	if !ok {
		// URN does not match any known pattern.
		return nil, &scim.ErrorSchemaNotFound
	}

	runtimeCtx := security.WithRuntimeContext(ctx)
	resolvedUserTypeName, svcErr := scim.ResolveUserTypeNameForSchemaURN(runtimeCtx, s.userTypeService, userTypeName)
	if svcErr != nil {
		return nil, svcErr
	}
	if resolvedUserTypeName == "" {
		s.logger.Debug(ctx, "User type not found for SCIM schema URN",
			log.String("urn", schemaURN),
			log.String("resolvedUserTypeName", userTypeName),
		)
		return nil, &scim.ErrorSchemaNotFound
	}

	et, svcErr := s.userTypeService.GetEntityTypeByName(
		runtimeCtx, entitytype.TypeCategoryUser, resolvedUserTypeName,
	)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, &scim.ErrorInternalServer
		}
		// User type not found or any other non-auth error → schema not found.
		s.logger.Debug(ctx, "User type not found for SCIM schema URN",
			log.String("urn", schemaURN),
			log.String("resolvedUserTypeName", resolvedUserTypeName),
		)
		return nil, &scim.ErrorSchemaNotFound
	}

	scimSchema, err := mapUserTypeToSCIMSchema(*et, baseURL)
	if err != nil {
		s.logger.Error(ctx, "Failed to map user type to SCIM schema",
			log.String("userTypeName", et.Name),
			log.Error(err),
		)
		return nil, &scim.ErrorInternalServer
	}

	return &scimSchema, nil
}

// ListResourceTypes returns all SCIM resource types supported by ThunderID.
// ThunderID exposes "User" and "Group" resource types. The User schemaExtensions
// array is built dynamically — one entry per registered user type.
func (s *scimDiscoveryService) ListResourceTypes(
	ctx context.Context, baseURL string,
) (SCIMResourceTypeListResponse, *tidcommon.ServiceError) {
	userRT, svcErr := s.buildUserResourceType(ctx, baseURL)
	if svcErr != nil {
		return SCIMResourceTypeListResponse{}, svcErr
	}

	groupRT := s.buildGroupResourceType(baseURL)

	resources := []SCIMResourceType{userRT, groupRT}
	return SCIMResourceTypeListResponse{
		Schemas:      []string{scim.SCIMListResponseSchemaURN},
		TotalResults: len(resources),
		StartIndex:   1,
		ItemsPerPage: len(resources),
		Resources:    resources,
	}, nil
}

// GetResourceType returns a single SCIM resource type by ID.
// Supported IDs are "User" and "Group" (case-insensitive). All others return 404.
func (s *scimDiscoveryService) GetResourceType(
	ctx context.Context, resourceTypeID string, baseURL string,
) (*SCIMResourceType, *tidcommon.ServiceError) {
	trimmed := strings.TrimSpace(resourceTypeID)
	switch {
	case strings.EqualFold(trimmed, scimResourceTypeUserID):
		rt, svcErr := s.buildUserResourceType(ctx, baseURL)
		if svcErr != nil {
			return nil, svcErr
		}
		return &rt, nil
	case strings.EqualFold(trimmed, scimResourceTypeGroupID):
		rt := s.buildGroupResourceType(baseURL)
		return &rt, nil
	default:
		s.logger.Debug(ctx, "SCIM ResourceType not found", log.String("id", resourceTypeID))
		return nil, &scim.ErrorResourceTypeNotFound
	}
}

// listUserTypeNames paginates through all user-category entity types and
// returns a flat slice of their names.
// This is the single authoritative pagination loop for user type name discovery.
// ListSchemas uses it to avoid duplicating pagination logic.
func (s *scimDiscoveryService) listUserTypeNames(ctx context.Context) ([]string, *tidcommon.ServiceError) {
	runtimeCtx := security.WithRuntimeContext(ctx)
	names := make([]string, 0, 16)
	offset := 0
	for {
		page, svcErr := s.userTypeService.GetEntityTypeList(
			runtimeCtx, entitytype.TypeCategoryUser, serverconst.MaxPageSize, offset, false,
		)
		if svcErr != nil {
			s.logger.Error(runtimeCtx, "Failed to list user types",
				log.Int("offset", offset), log.Any("error", svcErr))
			return nil, scim.MapEntityTypeServiceErrorToSCIM(svcErr)
		}

		for _, item := range page.Types {
			names = append(names, item.Name)
		}

		offset += len(page.Types)
		if offset >= page.TotalResults || len(page.Types) == 0 {
			break
		}
	}

	return names, nil
}

// resolveCoreUserEntityType resolves the ThunderID user type designated (via
// SCIMConfig.CoreUserTypeID, or the sole-user-type fallback) as the source of truth for the
// SCIM core User schema, and loads its full EntityType record (schema included). Returns a
// ServiceError with code scim.ErrorMissingCustomSchema.Code when no core user type can be
// determined — callers should treat that as "core schema unavailable," not a hard failure.
// Used by GetSchema, which has no pre-fetched user type list to reuse; ListSchemas uses
// coreUserTypeFromNames instead to avoid a second full pagination walk.
func (s *scimDiscoveryService) resolveCoreUserEntityType(
	ctx context.Context,
) (*entitytype.EntityType, *tidcommon.ServiceError) {
	runtimeCtx := security.WithRuntimeContext(ctx)
	name, svcErr := scim.ResolveCoreUserType(runtimeCtx, s.userTypeService, s.cfg.CoreUserTypeID)
	if svcErr != nil {
		return nil, svcErr
	}
	et, svcErr := s.userTypeService.GetEntityTypeByName(runtimeCtx, entitytype.TypeCategoryUser, name)
	if svcErr != nil {
		return nil, scim.MapEntityTypeServiceErrorToSCIM(svcErr)
	}
	return et, nil
}

// coreUserTypeFromNames resolves the designated core user type the same way as
// resolveCoreUserEntityType, but using names — the caller's already-fetched full list of
// configured user type names — instead of re-paginating GetEntityTypeList to determine
// whether exactly one user type exists.
func (s *scimDiscoveryService) coreUserTypeFromNames(
	ctx context.Context, names []string,
) (*entitytype.EntityType, *tidcommon.ServiceError) {
	runtimeCtx := security.WithRuntimeContext(ctx)

	if s.cfg.CoreUserTypeID != "" {
		et, svcErr := s.userTypeService.GetEntityType(
			runtimeCtx, entitytype.TypeCategoryUser, s.cfg.CoreUserTypeID, false)
		if svcErr != nil {
			return nil, scim.MapEntityTypeServiceErrorToSCIM(svcErr)
		}
		return et, nil
	}
	if len(names) != 1 {
		return nil, &scim.ErrorMissingCustomSchema
	}
	et, svcErr := s.userTypeService.GetEntityTypeByName(runtimeCtx, entitytype.TypeCategoryUser, names[0])
	if svcErr != nil {
		return nil, scim.MapEntityTypeServiceErrorToSCIM(svcErr)
	}
	return et, nil
}

// buildUserResourceType constructs the SCIM User ResourceType resource.
// The schemaExtensions array is built dynamically from all registered
// user type names.
// The core User schema URN is always the primary Schema field; each registered
// user-type contributes one required=false extension entry.
func (s *scimDiscoveryService) buildUserResourceType(
	ctx context.Context, baseURL string,
) (SCIMResourceType, *tidcommon.ServiceError) {
	location := fmt.Sprintf("%s%s/ResourceTypes/%s", baseURL, scim.SCIMBasePath, scimResourceTypeUserID)

	// Reuse the shared paginator — no duplicated pagination logic here.
	names, svcErr := s.listUserTypeNames(ctx)
	if svcErr != nil {
		return SCIMResourceType{}, svcErr
	}

	extensions := make([]scimResourceTypeSchemaExtension, 0, len(names))
	for _, name := range names {
		extensions = append(extensions, scimResourceTypeSchemaExtension{
			Schema:   scim.BuildSchemaURN(name),
			Required: false,
		})
	}

	return SCIMResourceType{
		Schemas:          []string{scimResourceTypeSchemaURN},
		ID:               scimResourceTypeUserID,
		Name:             scimResourceTypeUserName,
		Description:      scimResourceTypeUserDesc,
		Endpoint:         scimResourceTypeUserEndpoint,
		Schema:           scim.SCIMCoreUserSchemaURN,
		SchemaExtensions: extensions,
		Meta: scim.SCIMMeta{
			ResourceType: "ResourceType",
			Location:     location,
			// ResourceType definitions are server-managed and never mutated by clients.
			// Reuse the same stable timestamp constant used by ServiceProviderConfig.
			Created:      s.serverStartTime,
			LastModified: s.serverStartTime,
		},
	}, nil
}

// buildGroupResourceType constructs the static SCIM Group ResourceType resource.
// Groups have no dynamic schema extensions — the Group schema is the core RFC 7643 §4.2 schema.
func (s *scimDiscoveryService) buildGroupResourceType(baseURL string) SCIMResourceType {
	location := fmt.Sprintf("%s%s/ResourceTypes/%s", baseURL, scim.SCIMBasePath, scimResourceTypeGroupID)
	return SCIMResourceType{
		Schemas:          []string{scimResourceTypeSchemaURN},
		ID:               scimResourceTypeGroupID,
		Name:             scimResourceTypeGroupName,
		Description:      scimResourceTypeGroupDesc,
		Endpoint:         scimResourceTypeGroupEndpoint,
		Schema:           scim.SCIMCoreGroupSchemaURN,
		SchemaExtensions: []scimResourceTypeSchemaExtension{},
		Meta: scim.SCIMMeta{
			ResourceType: "ResourceType",
			Location:     location,
			Created:      s.serverStartTime,
			LastModified: s.serverStartTime,
		},
	}
}
