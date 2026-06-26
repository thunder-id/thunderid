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
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/user"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// SCIMServiceInterface defines the SCIM service operations.
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
	cfg               config.SCIMConfig

	// configVersion is a short deterministic hash of the SCIM config used
	// as the ETag/version value. It changes only when operator toggles a
	// capability flag — not on every server restart.
	configVersion string
}

// newSCIMService creates a new scimService instance.
func newSCIMService(
	userService user.UserServiceInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
	cfg config.SCIMConfig,
) *scimService {
	return &scimService{
		userService:       userService,
		entityTypeService: entityTypeService,
		cfg:               cfg,
		configVersion:     computeSCIMConfigVersion(cfg),
	}
}

// computeSCIMConfigVersion produces a stable weak ETag value from the SCIM
// config JSON. The format follows RFC 7232 weak validator convention: W/"<value>".
// It changes whenever an operator toggles a capability flag, ensuring SCIM
// clients can detect ServiceProviderConfig changes via conditional GET.
func computeSCIMConfigVersion(cfg config.SCIMConfig) string {
	b, err := json.Marshal(cfg)
	if err != nil {
		panic(fmt.Sprintf("scim: failed to marshal SCIM config for ETag generation: %v", err))
	}
	h := sha256.Sum256(b)
	return fmt.Sprintf("W/%q", hex.EncodeToString(h[:8]))
}

func (s *scimService) GetServiceProviderConfig(_ context.Context, baseURL string) SCIMServiceProviderConfig {
	location := fmt.Sprintf("%s%s/ServiceProviderConfig", baseURL, SCIMBasePath)

	meta := SCIMMeta{
		ResourceType: "ServiceProviderConfig",
		Created:      scimServiceProviderConfigCreated,
		LastModified: scimServiceProviderConfigCreated, // equals Created — resource never modified by users
		Location:     location,
	}

	// RFC 7643 §3.1: "version" is optional and subject to etag support.
	// Only include it when the server advertises ETag support.
	if s.cfg.ETagSupported {
		meta.Version = s.configVersion
	}

	return SCIMServiceProviderConfig{
		Schemas: []string{SCIMServiceProviderConfigSchemaURN},
		Patch:   SCIMSupportedFeature{Supported: s.cfg.PatchSupported},
		Bulk: SCIMBulkConfig{
			Supported:      s.cfg.BulkSupported,
			MaxOperations:  s.cfg.BulkMaxOperations,
			MaxPayloadSize: s.cfg.BulkMaxPayloadSize,
		},
		Filter: SCIMFilterConfig{
			Supported:  s.cfg.FilterSupported,
			MaxResults: s.cfg.FilterMaxResults,
		},
		ChangePassword: SCIMSupportedFeature{Supported: s.cfg.ChangePasswordSupported},
		Sort:           SCIMSupportedFeature{Supported: s.cfg.SortSupported},
		ETag:           SCIMSupportedFeature{Supported: s.cfg.ETagSupported},
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

// listUserEntityTypeNames paginates through all user-category entity types and
// returns a flat slice of their names.
//
// This is the single authoritative pagination loop for entity type name discovery.
// ListSchemas uses it to avoid duplicating pagination logic.
func (s *scimService) listUserEntityTypeNames(ctx context.Context) ([]string, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	names := make([]string, 0, 16)
	offset := 0
	for {
		page, svcErr := s.entityTypeService.GetEntityTypeList(
			ctx, entitytype.TypeCategoryUser, serverconst.MaxPageSize, offset, false,
		)
		if svcErr != nil {
			logger.Error(ctx, "Failed to list entity types",
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

// ListSchemas returns all SCIM schemas: the core User schema plus one extension
// schema per registered ThunderID user-type entity type.
func (s *scimService) ListSchemas(
	ctx context.Context, baseURL string,
) (SCIMSchemaListResponse, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	// --- 1. Collect all entity type names (single shared paginator) ---
	names, svcErr := s.listUserEntityTypeNames(ctx)
	if svcErr != nil {
		return SCIMSchemaListResponse{}, svcErr
	}

	// --- 2. Core User schema is always first ---
	schemas := make([]SCIMSchema, 0, 1+len(names))
	schemas = append(schemas, buildCoreUserSchema(baseURL))

	// --- 3. One extension schema per entity type ---
	for _, name := range names {
		et, svcErr := s.entityTypeService.GetEntityTypeByName(
			ctx, entitytype.TypeCategoryUser, name,
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

func (s *scimService) GetSchema(
	ctx context.Context, schemaURN string, baseURL string,
) (*SCIMSchema, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

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

	// --- 2. ThunderID extension schema (dynamic, from DB) ---
	userTypeName, ok := parseUserTypeFromSchemaURN(lowerURN)
	if !ok {
		// URN does not match any known pattern.
		return nil, &ErrorSchemaNotFound
	}

	entityTypeName, svcErr := s.resolveEntityTypeNameForSchemaURN(ctx, userTypeName)
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
		ctx, entitytype.TypeCategoryUser, entityTypeName,
	)
	if svcErr != nil {
		if svcErr.Code == tidcommon.ErrorUnauthorized.Code {
			// Propagate auth errors as-is — don't convert 401 to 404.
			return nil, svcErr
		}
		// Entity type not found or any other non-auth error → schema not found.
		logger.Debug(ctx, "Entity type not found for SCIM schema URN",
			log.String("urn", schemaURN),
			log.String("resolvedUserTypeName", entityTypeName),
		)
		return nil, &ErrorSchemaNotFound
	}

	scimSchema, err := mapEntityTypeToSCIMSchema(*et, baseURL)
	if err != nil {
		// The entity type exists but its schema JSON is malformed — this is a
		// server-side data integrity problem, not a client error.
		logger.Error(ctx, "Failed to map entity type to SCIM schema",
			log.String("entityTypeName", et.Name),
			log.Error(err),
		)
		return nil, &tidcommon.InternalServerError
	}

	return &scimSchema, nil
}

// ListResourceTypes returns all SCIM resource types supported by ThunderID.
// ThunderID only exposes a single "User" resource type. The schemaExtensions
// array is built dynamically — one entry per registered user-type entity type.
// RFC 7643 §6, RFC 7644 §4
func (s *scimService) ListResourceTypes(
	ctx context.Context, baseURL string,
) (SCIMResourceTypeListResponse, *tidcommon.ServiceError) {
	rt, svcErr := s.buildUserResourceType(ctx, baseURL)
	if svcErr != nil {
		return SCIMResourceTypeListResponse{}, svcErr
	}

	return SCIMResourceTypeListResponse{
		Schemas:      []string{SCIMListResponseSchemaURN},
		TotalResults: 1,
		StartIndex:   1,
		ItemsPerPage: 1,
		Resources:    []SCIMResourceType{rt},
	}, nil
}

// GetResourceType returns a single SCIM resource type by ID.
// The only supported ID is "User" (case-insensitive). All others return 404.
// RFC 7643 §6
func (s *scimService) GetResourceType(
	ctx context.Context, resourceTypeID string, baseURL string,
) (*SCIMResourceType, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	if !strings.EqualFold(strings.TrimSpace(resourceTypeID), scimResourceTypeUserID) {
		logger.Debug(ctx, "SCIM ResourceType not found", log.String("id", resourceTypeID))
		return nil, &ErrorResourceTypeNotFound
	}

	rt, svcErr := s.buildUserResourceType(ctx, baseURL)
	if svcErr != nil {
		return nil, svcErr
	}

	return &rt, nil
}

// buildUserResourceType constructs the SCIM User ResourceType resource.
// The schemaExtensions array is built dynamically by paginating through all
// user-type entity types — identical pagination pattern to ListSchemas.
// The core User schema URN is always the primary Schema field; each registered
// user-type contributes one required=false extension entry.
// RFC 7643 §6
func (s *scimService) buildUserResourceType(
	ctx context.Context, baseURL string,
) (SCIMResourceType, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	location := fmt.Sprintf("%s%s/ResourceTypes/%s", baseURL, SCIMBasePath, scimResourceTypeUserID)
	extensions := make([]SCIMResourceTypeSchemaExtension, 0, 8)

	offset := 0
	for {
		page, svcErr := s.entityTypeService.GetEntityTypeList(
			ctx, entitytype.TypeCategoryUser, serverconst.MaxPageSize, offset, false,
		)
		if svcErr != nil {
			logger.Error(ctx, "Failed to list entity types for SCIM ResourceType discovery",
				log.Int("offset", offset), log.Any("error", svcErr))
			return SCIMResourceType{}, svcErr
		}

		for _, item := range page.Types {
			extensions = append(extensions, SCIMResourceTypeSchemaExtension{
				// buildSchemaURN is defined in schema_mapper.go and shared across
				// all SCIM service methods — no duplication.
				Schema:   buildSchemaURN(item.Name),
				Required: false,
			})
		}

		offset += len(page.Types)
		if offset >= page.TotalResults || len(page.Types) == 0 {
			break
		}
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
			Created:      scimServiceProviderConfigCreated,
			LastModified: scimServiceProviderConfigCreated,
		},
	}, nil
}

func (s *scimService) resolveEntityTypeNameForSchemaURN(
	ctx context.Context, userTypeName string,
) (string, *tidcommon.ServiceError) {
	offset := 0
	for {
		page, svcErr := s.entityTypeService.GetEntityTypeList(
			ctx, entitytype.TypeCategoryUser, serverconst.MaxPageSize, offset, false,
		)
		if svcErr != nil {
			if svcErr.Code == tidcommon.ErrorUnauthorized.Code {
				return "", svcErr
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
