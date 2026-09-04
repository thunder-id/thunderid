// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/entitytype"
	scim "github.com/thunder-id/thunderid/internal/scim/common"
	scimconfig "github.com/thunder-id/thunderid/internal/scim/config"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const discoveryHandlerLoggerComponentName = "SCIMDiscoveryHandler"

// Handler handles SCIM discovery HTTP requests.
type Handler struct {
	svc     SCIMDiscoveryServiceInterface
	baseURL string
	logger  log.Logger
}

// NewHandler builds the discovery service internally and returns its handler,
// the sole exported entry point the root scim package's composition root needs.
func NewHandler(
	userTypeService entitytype.EntityTypeServiceInterface,
	cfg scimconfig.SCIMConfig,
	serverStartTime string,
	baseURL string,
) *Handler {
	svc := newSCIMDiscoveryService(userTypeService, cfg, serverStartTime)
	return &Handler{
		svc:     svc,
		baseURL: baseURL,
		logger:  *log.GetLogger().With(log.String(log.LoggerKeyComponentName, discoveryHandlerLoggerComponentName)),
	}
}

// HandleServiceProviderConfigGetRequest handles GET /scim/v2/ServiceProviderConfig.
func (sh *Handler) HandleServiceProviderConfigGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	config := sh.svc.GetServiceProviderConfig(ctx, sh.baseURL)
	scim.WriteSCIMSuccessResponse(ctx, w, http.StatusOK, config, discoveryHandlerLoggerComponentName)

	sh.logger.Debug(ctx, "SCIM ServiceProviderConfig GET response sent")
}

// HandleSchemaListRequest handles GET /scim/v2/Schemas.
// Returns all SCIM schemas: the core User schema plus one per ThunderID user type.
func (sh *Handler) HandleSchemaListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	listResp, svcErr := sh.svc.ListSchemas(ctx, sh.baseURL)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, discoveryHandlerLoggerComponentName)
		return
	}

	scim.WriteSCIMSuccessResponse(ctx, w, http.StatusOK, listResp, discoveryHandlerLoggerComponentName)
	sh.logger.Debug(ctx, "SCIM Schemas list response sent",
		log.Int("totalResults", listResp.TotalResults))
}

// HandleSchemaGetRequest handles GET /scim/v2/Schemas/{id}.
// The {id} path value is the full SCIM schema URN (e.g.
// urn:ietf:params:scim:schemas:core:2.0:User or a ThunderID extension URN).
func (sh *Handler) HandleSchemaGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	schemaURN := r.PathValue("id")
	if schemaURN == "" {
		scim.HandleSCIMError(w, r, &scim.ErrorSchemaNotFound, discoveryHandlerLoggerComponentName)
		return
	}

	schema, svcErr := sh.svc.GetSchema(ctx, schemaURN, sh.baseURL)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, discoveryHandlerLoggerComponentName)
		return
	}

	scim.WriteSCIMSuccessResponse(ctx, w, http.StatusOK, schema, discoveryHandlerLoggerComponentName)
	sh.logger.Debug(ctx, "SCIM Schema GET response sent", log.String("urn", schemaURN))
}

// HandleResourceTypeListRequest handles GET /scim/v2/ResourceTypes.
// Returns all SCIM resource types. ThunderID only exposes a single "User" resource type.
func (sh *Handler) HandleResourceTypeListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	listResp, svcErr := sh.svc.ListResourceTypes(ctx, sh.baseURL)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, discoveryHandlerLoggerComponentName)
		return
	}

	scim.WriteSCIMSuccessResponse(ctx, w, http.StatusOK, listResp, discoveryHandlerLoggerComponentName)
	sh.logger.Debug(ctx, "SCIM ResourceTypes list response sent",
		log.Int("totalResults", listResp.TotalResults))
}

// HandleResourceTypeGetRequest handles GET /scim/v2/ResourceTypes/{id}.
// The {id} path value is the resource type name — "User","Group" are the only supported value.
func (sh *Handler) HandleResourceTypeGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	resourceTypeID := r.PathValue("id")
	if resourceTypeID == "" {
		scim.HandleSCIMError(w, r, &scim.ErrorResourceTypeNotFound, discoveryHandlerLoggerComponentName)
		return
	}

	rt, svcErr := sh.svc.GetResourceType(ctx, resourceTypeID, sh.baseURL)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, discoveryHandlerLoggerComponentName)
		return
	}

	scim.WriteSCIMSuccessResponse(ctx, w, http.StatusOK, rt, discoveryHandlerLoggerComponentName)
	sh.logger.Debug(ctx, "SCIM ResourceType GET response sent", log.String("id", resourceTypeID))
}
