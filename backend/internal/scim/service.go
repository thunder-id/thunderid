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
	"github.com/thunder-id/thunderid/internal/user"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// SCIMServiceInterface defines the SCIM service operations.
type SCIMServiceInterface interface {
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

// scimService coordinates SCIM operations, delegating user and entity type
// operations to existing ThunderID services.
type scimService struct {
	userService       user.UserServiceInterface
	entityTypeService entitytype.EntityTypeServiceInterface
	cfg               scimconfig.SCIMConfig

	// configVersion is a short deterministic hash of the SCIM config used as the
	// ETag value for ServiceProviderConfig. Computed once at startup and immutable
	// for the lifetime of the service instance; differs across deployments when an
	// operator changes a capability flag.
	configVersion string
	logger        *log.Logger
}

// newSCIMService creates a new scimService instance.
func newSCIMService(
	userService user.UserServiceInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
	cfg scimconfig.SCIMConfig,
) *scimService {
	return &scimService{
		userService:       userService,
		entityTypeService: entityTypeService,
		cfg:               cfg,
		configVersion:     computeSCIMConfigVersion(cfg),
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "SCIMService")),
	}
}

// computeSCIMConfigVersion produces a stable weak ETag value from the SCIM
// config JSON. The format follows RFC 7232 weak validator convention: W/"<value>".
// It changes whenever an operator toggles a capability flag, ensuring SCIM
// clients can detect ServiceProviderConfig changes via conditional GET.
func computeSCIMConfigVersion(cfg scimconfig.SCIMConfig) string {
	state := struct {
		scimconfig.SCIMConfig
		PatchSupported          bool
		BulkSupported           bool
		BulkMaxOperations       int
		BulkMaxPayloadSize      int
		FilterSupported         bool
		FilterMaxResults        int
		ChangePasswordSupported bool
		SortSupported           bool
		ETagSupported           bool
	}{
		SCIMConfig:              cfg,
		PatchSupported:          scimconfig.PatchSupported,
		BulkSupported:           scimconfig.BulkSupported,
		BulkMaxOperations:       scimconfig.BulkMaxOperations,
		BulkMaxPayloadSize:      scimconfig.BulkMaxPayloadSize,
		FilterSupported:         scimconfig.FilterSupported,
		FilterMaxResults:        scimconfig.FilterMaxResults,
		ChangePasswordSupported: scimconfig.ChangePasswordSupported,
		SortSupported:           scimconfig.SortSupported,
		ETagSupported:           scimconfig.ETagSupported,
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
func (s *scimService) GetServiceProviderConfig(_ context.Context, baseURL string) SCIMServiceProviderConfig {
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
func (s *scimService) ListSchemas(
	ctx context.Context, baseURL string,
) (SCIMSchemaListResponse, *tidcommon.ServiceError) {
	logger := s.logger

	// --- 1. Collect all entity type names (single shared paginator) ---
	names, svcErr := s.listUserEntityTypeNames(ctx)
	if svcErr != nil {
		return SCIMSchemaListResponse{}, svcErr
	}

	// --- 2. Core User and Group schemas are always included ---
	schemas := make([]SCIMSchema, 0, 2+len(names))
	schemas = append(schemas, buildCoreUserSchema(baseURL))
	schemas = append(schemas, buildCoreGroupSchema(baseURL))

	// --- 3. One extension schema per entity type ---
	runtimeCtx := security.WithRuntimeContext(ctx)
	for _, name := range names {
		et, svcErr := s.entityTypeService.GetEntityTypeByName(
			runtimeCtx, entitytype.TypeCategoryUser, name,
		)
		if svcErr != nil {
			logger.Warn(ctx, "Failed to load entity type for SCIM schema list, skipping",
				log.String("entityTypeName", name),
				log.Any("error", svcErr),
			)
			continue
		}

		scimSchema, err := mapEntityTypeToSCIMSchema(*et, baseURL)
		if err != nil {
			logger.Warn(ctx, "Failed to map entity type to SCIM schema, skipping",
				log.String("entityTypeName", name),
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
func (s *scimService) GetSchema(
	ctx context.Context, schemaURN string, baseURL string,
) (*SCIMSchema, *tidcommon.ServiceError) {
	logger := s.logger

	trimmedURN := strings.TrimSpace(schemaURN)
	if trimmedURN == "" {
		return nil, &ErrorSchemaNotFound
	}

	// Case-insensitive URN comparison per RFC 7643 §1.2 which states schema URNs
	// "SHOULD" be compared case-insensitively.
	lowerURN := strings.ToLower(trimmedURN)

	// --- 1. Core User schema (static, RFC 7643 §4.1) ---
	if lowerURN == strings.ToLower(SCIMCoreUserSchemaURN) {
		schema := buildCoreUserSchema(baseURL)
		schema.Schemas = []string{SCIMSchemaSchemaURN}
		return &schema, nil
	}

	// --- 2. Core Group schema (static, RFC 7643 §4.2) ---
	if lowerURN == strings.ToLower(SCIMCoreGroupSchemaURN) {
		schema := buildCoreGroupSchema(baseURL)
		schema.Schemas = []string{SCIMSchemaSchemaURN}
		return &schema, nil
	}

	// --- 3. ThunderID extension schema (dynamic, from DB) ---
	userTypeName, ok := parseUserTypeFromSchemaURN(lowerURN)
	if !ok {
		// URN does not match any known pattern.
		return nil, &ErrorSchemaNotFound
	}

	runtimeCtx := security.WithRuntimeContext(ctx)
	entityTypeName, svcErr := resolveEntityTypeNameForSchemaURN(runtimeCtx, s.entityTypeService, userTypeName)
	if svcErr != nil {
		return nil, svcErr
	}
	if entityTypeName == "" {
		logger.Debug(ctx, "Entity type not found for SCIM schema URN",
			log.String("urn", schemaURN),
			log.String("resolvedUserTypeName", userTypeName),
		)
		return nil, &ErrorSchemaNotFound
	}

	et, svcErr := s.entityTypeService.GetEntityTypeByName(
		runtimeCtx, entitytype.TypeCategoryUser, entityTypeName,
	)
	if svcErr != nil {
		// Entity type not found or any other non-auth error → schema not found.
		logger.Debug(ctx, "Entity type not found for SCIM schema URN",
			log.String("urn", schemaURN),
			log.String("resolvedUserTypeName", entityTypeName),
		)
		return nil, &ErrorSchemaNotFound
	}

	scimSchema, err := mapEntityTypeToSCIMSchema(*et, baseURL)
	if err != nil {
		logger.Error(ctx, "Failed to map entity type to SCIM schema",
			log.String("entityTypeName", et.Name),
			log.Error(err),
		)
		return nil, &ErrorInternalServer
	}

	return &scimSchema, nil
}

// ListResourceTypes returns all SCIM resource types supported by ThunderID.
// ThunderID exposes "User" and "Group" resource types. The User schemaExtensions
// array is built dynamically — one entry per registered user-type entity type.
func (s *scimService) ListResourceTypes(
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
func (s *scimService) GetResourceType(
	ctx context.Context, resourceTypeID string, baseURL string,
) (*SCIMResourceType, *tidcommon.ServiceError) {
	logger := s.logger

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

// listUserEntityTypeNames paginates through all user-category entity types and
// returns a flat slice of their names.
// This is the single authoritative pagination loop for entity type name discovery.
// ListSchemas uses it to avoid duplicating pagination logic.
func (s *scimService) listUserEntityTypeNames(ctx context.Context) ([]string, *tidcommon.ServiceError) {
	runtimeCtx := security.WithRuntimeContext(ctx)
	logger := s.logger
	names := make([]string, 0, 16)
	offset := 0
	for {
		page, svcErr := s.entityTypeService.GetEntityTypeList(
			runtimeCtx, entitytype.TypeCategoryUser, serverconst.MaxPageSize, offset, false,
		)
		if svcErr != nil {
			logger.Error(runtimeCtx, "Failed to list entity types",
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
// user-type entity type names.
// The core User schema URN is always the primary Schema field; each registered
// user-type contributes one required=false extension entry.
func (s *scimService) buildUserResourceType(
	ctx context.Context, baseURL string,
) (SCIMResourceType, *tidcommon.ServiceError) {
	location := fmt.Sprintf("%s%s/ResourceTypes/%s", baseURL, SCIMBasePath, scimResourceTypeUserID)

	// Reuse the shared paginator — no duplicated pagination logic here.
	names, svcErr := s.listUserEntityTypeNames(ctx)
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

// resolveEntityTypeNameForSchemaURN searches all user-type entity types for one
// whose name matches userTypeName (case-insensitive). Returns the canonical name
// and nil on success, or empty string and nil if no match is found.
func resolveEntityTypeNameForSchemaURN(
	ctx context.Context, entityTypeService entitytype.EntityTypeServiceInterface, userTypeName string,
) (string, *tidcommon.ServiceError) {
	offset := 0
	for {
		page, svcErr := entityTypeService.GetEntityTypeList(
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
