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
		ctx context.Context, baseURL string, startIndex, count int,
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

// scimDiscoveryService implements SCIMDiscoveryServiceInterface.
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
		logger: *log.GetLogger().With(
			log.String(log.LoggerKeyComponentName, discoveryServiceLoggerComponentName)),
	}
}

// GetServiceProviderConfig returns the SCIM ServiceProviderConfig resource.
func (s *scimDiscoveryService) GetServiceProviderConfig(_ context.Context, baseURL string) SCIMServiceProviderConfig {
	location := fmt.Sprintf(scimServiceProviderConfigLocationFmt, baseURL, scim.SCIMBasePath)

	meta := scim.SCIMMeta{
		ResourceType: scimMetaResourceTypeServiceProviderConfig,
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

// ListSchemas returns a paginated list of SCIM schemas.
func (s *scimDiscoveryService) ListSchemas(
	ctx context.Context, baseURL string, startIndex, count int,
) (SCIMSchemaListResponse, *tidcommon.ServiceError) {
	// --- 1. Resolve the core user type (if any) and the total registered user-type count,
	// using at most one entity-type list query. ---
	coreType, dynamicTotal, svcErr := s.resolveCoreUserTypeAndTotal(ctx)
	if svcErr != nil {
		return SCIMSchemaListResponse{}, svcErr
	}

	// --- 2. Static schemas: Core User and Enterprise User are included only if a core user type
	// is resolvable and loadable. Core Group has no ThunderID user-type backing and is always
	// included. ---
	staticSchemas := make([]SCIMSchema, 0, 3)
	if coreType != nil {
		if coreSchema, err := buildCoreUserSchema(baseURL, *coreType); err != nil {
			s.logger.Warn(ctx, "Failed to build SCIM core User schema, omitting", log.Error(err))
		} else {
			staticSchemas = append(staticSchemas, coreSchema)
		}
		if entSchema, err := buildEnterpriseUserSchema(baseURL, *coreType); err != nil {
			s.logger.Warn(ctx, "Failed to build SCIM Enterprise User schema, omitting", log.Error(err))
		} else if len(entSchema.Attributes) > 0 {
			staticSchemas = append(staticSchemas, entSchema)
		}
	}
	staticSchemas = append(staticSchemas, buildCoreGroupSchema(baseURL))

	totalResults := len(staticSchemas) + dynamicTotal

	// --- 3. Slice the requested window: static schemas first, then only the user-type
	// extension schemas that fall within the remaining window. ---
	staticStart := min(startIndex-1, len(staticSchemas))
	staticEnd := min(staticStart+count, len(staticSchemas))
	resources := append([]SCIMSchema{}, staticSchemas[staticStart:staticEnd]...)

	remaining := count - (staticEnd - staticStart)
	entityOffset := max(0, startIndex-1-len(staticSchemas))
	if remaining > 0 && entityOffset < dynamicTotal {
		runtimeCtx := security.WithRuntimeContext(ctx)
		page, svcErr := s.userTypeService.GetEntityTypeList(
			runtimeCtx, entitytype.TypeCategoryUser, remaining, entityOffset, false,
		)
		if svcErr != nil {
			s.logger.Error(ctx, "Failed to list user types for SCIM schema page",
				log.Int("offset", entityOffset), log.Any("error", svcErr))
			return SCIMSchemaListResponse{}, scim.BuildUserTypeErrorToSCIM(svcErr)
		}

		for _, item := range page.Types {
			et, svcErr := s.userTypeService.GetEntityTypeByName(
				runtimeCtx, entitytype.TypeCategoryUser, item.Name,
			)
			if svcErr != nil {
				s.logger.Warn(ctx, "Failed to load user type for SCIM schema list, skipping",
					log.String("userTypeName", item.Name),
					log.Any("error", svcErr),
				)
				continue
			}

			scimSchema, err := mapUserTypeToSCIMSchema(*et, baseURL)
			if err != nil {
				s.logger.Warn(ctx, "Failed to map user type to SCIM schema, skipping",
					log.String("userTypeName", item.Name),
					log.Error(err),
				)
				continue
			}
			resources = append(resources, scimSchema)
		}
	}

	return SCIMSchemaListResponse{
		Schemas:      []string{scim.SCIMListResponseSchemaURN},
		TotalResults: totalResults,
		StartIndex:   startIndex,
		ItemsPerPage: len(resources),
		Resources:    resources,
	}, nil
}

// GetSchema returns a single SCIM Schema resource by URN.
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
				return nil, &tidcommon.InternalServerError
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
			return nil, &tidcommon.InternalServerError
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

	// --- 3. Enterprise User schema extension (RFC 7643 §4.3) ---
	if strings.EqualFold(trimmedURN, scim.SCIMEnterpriseUserSchemaURN) {
		coreType, svcErr := s.resolveCoreUserEntityType(ctx)
		if svcErr != nil {
			s.logger.Debug(ctx, "Core user type unavailable for SCIM Enterprise User schema URN",
				log.Any("error", svcErr))
			return nil, &scim.ErrorSchemaNotFound
		}
		schema, err := buildEnterpriseUserSchema(baseURL, *coreType)
		if err != nil {
			s.logger.Error(ctx, "Failed to build SCIM Enterprise User schema", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		if len(schema.Attributes) == 0 {
			return nil, &scim.ErrorSchemaNotFound
		}
		schema.Schemas = []string{scimSchemaSchemaURN}
		return &schema, nil
	}

	// --- 4. ThunderID extension schema (dynamic, from DB) ---
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
			return nil, &tidcommon.InternalServerError
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
		return nil, &tidcommon.InternalServerError
	}

	return &scimSchema, nil
}

// ListResourceTypes returns all SCIM resource types supported by ThunderID.
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

// listUserTypeNames paginates through all user-category entity types and returns their names.
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
			return nil, scim.BuildUserTypeErrorToSCIM(svcErr)
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

// resolveCoreUserEntityType resolves the designated core user type and loads its EntityType record.
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
		return nil, scim.BuildUserTypeErrorToSCIM(svcErr)
	}
	return et, nil
}

// resolveCoreUserTypeAndTotal resolves the designated core user type and the total user-type count.
func (s *scimDiscoveryService) resolveCoreUserTypeAndTotal(
	ctx context.Context,
) (*entitytype.EntityType, int, *tidcommon.ServiceError) {
	runtimeCtx := security.WithRuntimeContext(ctx)

	page, svcErr := s.userTypeService.GetEntityTypeList(
		runtimeCtx, entitytype.TypeCategoryUser, 1, 0, false,
	)
	if svcErr != nil {
		return nil, 0, scim.BuildUserTypeErrorToSCIM(svcErr)
	}

	if s.cfg.CoreUserTypeID != "" {
		et, svcErr := s.userTypeService.GetEntityType(
			runtimeCtx, entitytype.TypeCategoryUser, s.cfg.CoreUserTypeID, false)
		if svcErr != nil {
			s.logger.Debug(ctx, "Core user type unavailable, omitting SCIM core User schema",
				log.Any("error", scim.BuildUserTypeErrorToSCIM(svcErr)))
			return nil, page.TotalResults, nil
		}
		return et, page.TotalResults, nil
	}

	if page.TotalResults != 1 {
		s.logger.Debug(ctx, "Core user type unavailable, omitting SCIM core User schema",
			log.Any("error", &scim.ErrorMissingCustomSchema))
		return nil, page.TotalResults, nil
	}
	et, svcErr := s.userTypeService.GetEntityTypeByName(
		runtimeCtx, entitytype.TypeCategoryUser, page.Types[0].Name,
	)
	if svcErr != nil {
		s.logger.Debug(ctx, "Core user type unavailable, omitting SCIM core User schema",
			log.Any("error", scim.BuildUserTypeErrorToSCIM(svcErr)))
		return nil, page.TotalResults, nil
	}
	return et, page.TotalResults, nil
}

// buildUserResourceType constructs the SCIM User ResourceType resource.
func (s *scimDiscoveryService) buildUserResourceType(
	ctx context.Context, baseURL string,
) (SCIMResourceType, *tidcommon.ServiceError) {
	location := fmt.Sprintf(scimResourceTypeLocationFmt, baseURL, scim.SCIMBasePath, scimResourceTypeUserID)

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
			ResourceType: scimMetaResourceTypeResourceType,
			Location:     location,
			// ResourceType definitions are server-managed and never mutated by clients.
			// Reuse the same stable timestamp constant used by ServiceProviderConfig.
			Created:      s.serverStartTime,
			LastModified: s.serverStartTime,
		},
	}, nil
}

// buildGroupResourceType constructs the static SCIM Group ResourceType resource.
func (s *scimDiscoveryService) buildGroupResourceType(baseURL string) SCIMResourceType {
	location := fmt.Sprintf(scimResourceTypeLocationFmt, baseURL, scim.SCIMBasePath, scimResourceTypeGroupID)
	return SCIMResourceType{
		Schemas:          []string{scimResourceTypeSchemaURN},
		ID:               scimResourceTypeGroupID,
		Name:             scimResourceTypeGroupName,
		Description:      scimResourceTypeGroupDesc,
		Endpoint:         scimResourceTypeGroupEndpoint,
		Schema:           scim.SCIMCoreGroupSchemaURN,
		SchemaExtensions: []scimResourceTypeSchemaExtension{},
		Meta: scim.SCIMMeta{
			ResourceType: scimMetaResourceTypeResourceType,
			Location:     location,
			Created:      s.serverStartTime,
			LastModified: s.serverStartTime,
		},
	}
}
