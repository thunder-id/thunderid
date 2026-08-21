// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package scim

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/thunder-id/thunderid/internal/entitytype"
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

const serviceLoggerComponentName = "SCIMService"

// scimDiscoveryService coordinates SCIM discovery operations (ServiceProviderConfig,
// Schemas, ResourceTypes), delegating user type operations to existing ThunderID services.
type scimDiscoveryService struct {
	userTypeService entitytype.EntityTypeServiceInterface
	cfg             scimconfig.SCIMConfig

	// configVersion is a short deterministic hash of the SCIM config used as the
	// ETag value for ServiceProviderConfig. Computed once at startup and immutable
	// for the lifetime of the service instance; differs across deployments when an
	// operator changes a capability flag.
	configVersion string
}

// newSCIMDiscoveryService creates a new scimDiscoveryService instance.
func newSCIMDiscoveryService(
	userTypeService entitytype.EntityTypeServiceInterface,
	cfg scimconfig.SCIMConfig,
) *scimDiscoveryService {
	return &scimDiscoveryService{
		userTypeService: userTypeService,
		cfg:             cfg,
		configVersion:   computeSCIMConfigVersion(cfg),
	}
}

// computeSCIMConfigVersion produces a stable weak ETag value from the SCIM
// config JSON. The format follows RFC 7232 weak validator convention: W/"<value>".
// It changes whenever an operator toggles a capability flag, ensuring SCIM
// clients can detect ServiceProviderConfig changes via conditional GET.
func computeSCIMConfigVersion(cfg scimconfig.SCIMConfig) string {
	state := struct {
		scimconfig.SCIMConfig
		PatchSupported            bool
		BulkSupported             bool
		BulkMaxOperations         int
		BulkMaxPayloadSize        int
		FilterSupported           bool
		FilterMaxResults          int
		ChangePasswordSupported   bool
		SortSupported             bool
		ETagSupported             bool
		PaginationCursorSupported bool
		PaginationIndexSupported  bool
		PaginationDefaultMethod   string
		PaginationDefaultPageSize int
		PaginationMaxPageSize     int
	}{
		SCIMConfig:                cfg,
		PatchSupported:            scimconfig.PatchSupported,
		BulkSupported:             scimconfig.BulkSupported,
		BulkMaxOperations:         scimconfig.BulkMaxOperations,
		BulkMaxPayloadSize:        scimconfig.BulkMaxPayloadSize,
		FilterSupported:           scimconfig.FilterSupported,
		FilterMaxResults:          scimconfig.FilterMaxResults,
		ChangePasswordSupported:   scimconfig.ChangePasswordSupported,
		SortSupported:             scimconfig.SortSupported,
		ETagSupported:             scimconfig.ETagSupported,
		PaginationCursorSupported: scimconfig.PaginationCursorSupported,
		PaginationIndexSupported:  scimconfig.PaginationIndexSupported,
		PaginationDefaultMethod:   scimconfig.PaginationDefaultMethod,
		PaginationDefaultPageSize: scimconfig.PaginationDefaultPageSize,
		PaginationMaxPageSize:     scimconfig.PaginationMaxPageSize,
	}

	b, err := json.Marshal(state)
	if err != nil {
		panic(fmt.Sprintf("scim: failed to marshal SCIM config for ETag generation: %v", err))
	}
	h := sha256.Sum256(b)
	return fmt.Sprintf("W/%q", hex.EncodeToString(h[:8]))
}

// GetServiceProviderConfig returns the SCIM ServiceProviderConfig resource
// describing the server's supported capabilities per RFC 7643 §5.
func (s *scimDiscoveryService) GetServiceProviderConfig(_ context.Context, baseURL string) SCIMServiceProviderConfig {
	location := fmt.Sprintf("%s%s/ServiceProviderConfig", baseURL, SCIMBasePath)

	meta := SCIMMeta{
		ResourceType: "ServiceProviderConfig",
		Created:      scimServerStartTime,
		LastModified: scimServerStartTime, // equals Created — resource never modified by users
		Location:     location,
	}

	// RFC 7643 §3.1: "version" is optional and subject to etag support.
	// Only include it when the server advertises ETag support.
	if scimconfig.ETagSupported {
		meta.Version = s.configVersion
	}

	return SCIMServiceProviderConfig{
		Schemas: []string{SCIMServiceProviderConfigSchemaURN},
		Patch:   SCIMSupportedFeature{Supported: scimconfig.PatchSupported},
		Bulk: SCIMBulkConfig{
			Supported:      scimconfig.BulkSupported,
			MaxOperations:  scimconfig.BulkMaxOperations,
			MaxPayloadSize: scimconfig.BulkMaxPayloadSize,
		},
		Filter: SCIMFilterConfig{
			Supported:  scimconfig.FilterSupported,
			MaxResults: scimconfig.FilterMaxResults,
		},
		ChangePassword: SCIMSupportedFeature{Supported: scimconfig.ChangePasswordSupported},
		Sort:           SCIMSupportedFeature{Supported: scimconfig.SortSupported},
		ETag:           SCIMSupportedFeature{Supported: scimconfig.ETagSupported},
		Pagination: SCIMPaginationConfig{
			Cursor:                  scimconfig.PaginationCursorSupported,
			Index:                   scimconfig.PaginationIndexSupported,
			DefaultPaginationMethod: scimconfig.PaginationDefaultMethod,
			DefaultPageSize:         scimconfig.PaginationDefaultPageSize,
			MaxPageSize:             scimconfig.PaginationMaxPageSize,
		},
		AuthenticationSchemes: []SCIMAuthenticationScheme{
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
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, serviceLoggerComponentName))

	// --- 1. Collect all user type names (single shared paginator) ---
	names, svcErr := s.listUserTypeNames(ctx)
	if svcErr != nil {
		return SCIMSchemaListResponse{}, svcErr
	}

	// --- 2. Core User and Group schemas are always included ---
	schemas := make([]SCIMSchema, 0, 2+len(names))
	schemas = append(schemas, buildCoreUserSchema(baseURL))
	schemas = append(schemas, buildCoreGroupSchema(baseURL))

	// --- 3. One extension schema per user type ---
	runtimeCtx := security.WithRuntimeContext(ctx)
	for _, name := range names {
		et, svcErr := s.userTypeService.GetEntityTypeByName(
			runtimeCtx, entitytype.TypeCategoryUser, name,
		)
		if svcErr != nil {
			logger.Warn(ctx, "Failed to load user type for SCIM schema list, skipping",
				log.String("userTypeName", name),
				log.Any("error", svcErr),
			)
			continue
		}

		scimSchema, err := mapUserTypeToSCIMSchema(*et, baseURL)
		if err != nil {
			logger.Warn(ctx, "Failed to map user type to SCIM schema, skipping",
				log.String("userTypeName", name),
				log.Error(err),
			)
			continue
		}
		schemas = append(schemas, scimSchema)
	}

	return SCIMSchemaListResponse{
		Schemas:      []string{SCIMListResponseSchemaURN},
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
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, serviceLoggerComponentName))

	trimmedURN := strings.TrimSpace(schemaURN)
	if trimmedURN == "" {
		return nil, &ErrorSchemaNotFound
	}

	// Case-insensitive URN comparison per RFC 7643 §1.2 which states schema URNs
	// "SHOULD" be compared case-insensitively.
	// --- 1. Core User schema (static, RFC 7643 §4.1) ---
	if strings.EqualFold(trimmedURN, SCIMCoreUserSchemaURN) {
		schema := buildCoreUserSchema(baseURL)
		schema.Schemas = []string{SCIMSchemaSchemaURN}
		return &schema, nil
	}

	// --- 2. Core Group schema (static, RFC 7643 §4.2) ---
	if strings.EqualFold(trimmedURN, SCIMCoreGroupSchemaURN) {
		schema := buildCoreGroupSchema(baseURL)
		schema.Schemas = []string{SCIMSchemaSchemaURN}
		return &schema, nil
	}

	// --- 3. ThunderID extension schema (dynamic, from DB) ---
	userTypeName, ok := parseUserTypeFromSchemaURN(trimmedURN)
	if !ok {
		// URN does not match any known pattern.
		return nil, &ErrorSchemaNotFound
	}

	runtimeCtx := security.WithRuntimeContext(ctx)
	resolvedUserTypeName, svcErr := resolveUserTypeNameForSchemaURN(runtimeCtx, s.userTypeService, userTypeName)
	if svcErr != nil {
		return nil, svcErr
	}
	if resolvedUserTypeName == "" {
		logger.Debug(ctx, "User type not found for SCIM schema URN",
			log.String("urn", schemaURN),
			log.String("resolvedUserTypeName", userTypeName),
		)
		return nil, &ErrorSchemaNotFound
	}

	et, svcErr := s.userTypeService.GetEntityTypeByName(
		runtimeCtx, entitytype.TypeCategoryUser, resolvedUserTypeName,
	)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, &ErrorInternalServer
		}
		// User type not found or any other non-auth error → schema not found.
		logger.Debug(ctx, "User type not found for SCIM schema URN",
			log.String("urn", schemaURN),
			log.String("resolvedUserTypeName", resolvedUserTypeName),
		)
		return nil, &ErrorSchemaNotFound
	}

	scimSchema, err := mapUserTypeToSCIMSchema(*et, baseURL)
	if err != nil {
		logger.Error(ctx, "Failed to map user type to SCIM schema",
			log.String("userTypeName", et.Name),
			log.Error(err),
		)
		return nil, &ErrorInternalServer
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

	groupRT := buildGroupResourceType(baseURL)

	resources := []SCIMResourceType{userRT, groupRT}
	return SCIMResourceTypeListResponse{
		Schemas:      []string{SCIMListResponseSchemaURN},
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
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, serviceLoggerComponentName))

	trimmed := strings.TrimSpace(resourceTypeID)
	switch {
	case strings.EqualFold(trimmed, scimResourceTypeUserID):
		rt, svcErr := s.buildUserResourceType(ctx, baseURL)
		if svcErr != nil {
			return nil, svcErr
		}
		return &rt, nil
	case strings.EqualFold(trimmed, scimResourceTypeGroupID):
		rt := buildGroupResourceType(baseURL)
		return &rt, nil
	default:
		logger.Debug(ctx, "SCIM ResourceType not found", log.String("id", resourceTypeID))
		return nil, &ErrorResourceTypeNotFound
	}
}

// listUserTypeNames paginates through all user-category entity types and
// returns a flat slice of their names.
// This is the single authoritative pagination loop for user type name discovery.
// ListSchemas uses it to avoid duplicating pagination logic.
func (s *scimDiscoveryService) listUserTypeNames(ctx context.Context) ([]string, *tidcommon.ServiceError) {
	runtimeCtx := security.WithRuntimeContext(ctx)
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, serviceLoggerComponentName))
	names := make([]string, 0, 16)
	offset := 0
	for {
		page, svcErr := s.userTypeService.GetEntityTypeList(
			runtimeCtx, entitytype.TypeCategoryUser, serverconst.MaxPageSize, offset, false,
		)
		if svcErr != nil {
			logger.Error(runtimeCtx, "Failed to list user types",
				log.Int("offset", offset), log.Any("error", svcErr))
			return nil, svcErr
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

// buildUserResourceType constructs the SCIM User ResourceType resource.
// The schemaExtensions array is built dynamically from all registered
// user type names.
// The core User schema URN is always the primary Schema field; each registered
// user-type contributes one required=false extension entry.
func (s *scimDiscoveryService) buildUserResourceType(
	ctx context.Context, baseURL string,
) (SCIMResourceType, *tidcommon.ServiceError) {
	location := fmt.Sprintf("%s%s/ResourceTypes/%s", baseURL, SCIMBasePath, scimResourceTypeUserID)

	// Reuse the shared paginator — no duplicated pagination logic here.
	names, svcErr := s.listUserTypeNames(ctx)
	if svcErr != nil {
		return SCIMResourceType{}, svcErr
	}

	extensions := make([]SCIMResourceTypeSchemaExtension, 0, len(names))
	for _, name := range names {
		extensions = append(extensions, SCIMResourceTypeSchemaExtension{
			Schema:   buildSchemaURN(name),
			Required: false,
		})
	}

	return SCIMResourceType{
		Schemas:          []string{SCIMResourceTypeSchemaURN},
		ID:               scimResourceTypeUserID,
		Name:             scimResourceTypeUserName,
		Description:      scimResourceTypeUserDesc,
		Endpoint:         scimResourceTypeUserEndpoint,
		Schema:           SCIMCoreUserSchemaURN,
		SchemaExtensions: extensions,
		Meta: SCIMMeta{
			ResourceType: "ResourceType",
			Location:     location,
			// ResourceType definitions are server-managed and never mutated by clients.
			// Reuse the same stable timestamp constant used by ServiceProviderConfig.
			Created:      scimServerStartTime,
			LastModified: scimServerStartTime,
		},
	}, nil
}

// buildGroupResourceType constructs the static SCIM Group ResourceType resource.
// Groups have no dynamic schema extensions — the Group schema is the core RFC 7643 §4.2 schema.
func buildGroupResourceType(baseURL string) SCIMResourceType {
	location := fmt.Sprintf("%s%s/ResourceTypes/%s", baseURL, SCIMBasePath, scimResourceTypeGroupID)
	return SCIMResourceType{
		Schemas:          []string{SCIMResourceTypeSchemaURN},
		ID:               scimResourceTypeGroupID,
		Name:             scimResourceTypeGroupName,
		Description:      scimResourceTypeGroupDesc,
		Endpoint:         scimResourceTypeGroupEndpoint,
		Schema:           SCIMCoreGroupSchemaURN,
		SchemaExtensions: []SCIMResourceTypeSchemaExtension{},
		Meta: SCIMMeta{
			ResourceType: "ResourceType",
			Location:     location,
			Created:      scimServerStartTime,
			LastModified: scimServerStartTime,
		},
	}
}

// resolveUserTypeNameForSchemaURN searches all user types for one
// whose name matches userTypeName (case-insensitive). Returns the resolved,
// correctly-cased name and nil on success, or empty string and nil if no match is found.
func resolveUserTypeNameForSchemaURN(
	ctx context.Context, userTypeService entitytype.EntityTypeServiceInterface, userTypeName string,
) (string, *tidcommon.ServiceError) {
	offset := 0
	for {
		page, svcErr := userTypeService.GetEntityTypeList(
			ctx, entitytype.TypeCategoryUser, serverconst.MaxPageSize, offset, false,
		)
		if svcErr != nil {
			if svcErr.Type == tidcommon.ServerErrorType {
				return "", &ErrorInternalServer
			}
			return "", &ErrorSchemaNotFound
		}

		for _, item := range page.Types {
			if strings.EqualFold(item.Name, userTypeName) {
				return item.Name, nil
			}
		}

		offset += len(page.Types)
		if offset >= page.TotalResults || len(page.Types) == 0 {
			return "", nil
		}
	}
}

// resolveDefaultUserTypeName returns the sole configured user type's
// resolved name, for SCIM payloads that carry only core attributes and omit
// the ThunderID extension URN. Errors if zero or more than one user type is
// configured, since the default type is then ambiguous.
func resolveDefaultUserTypeName(
	ctx context.Context, userTypeService entitytype.EntityTypeServiceInterface,
) (string, *tidcommon.ServiceError) {
	page, svcErr := userTypeService.GetEntityTypeList(
		ctx, entitytype.TypeCategoryUser, serverconst.MaxPageSize, 0, false)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return "", &ErrorInternalServer
		}
		return "", &ErrorMissingCustomSchema
	}
	if page.TotalResults != 1 || len(page.Types) != 1 {
		return "", &ErrorMissingCustomSchema
	}
	return page.Types[0].Name, nil
}
