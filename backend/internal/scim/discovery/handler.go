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

// NewHandler creates the discovery service and returns its handler.
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
	scim.WriteSCIMSuccessResponse(ctx, w, http.StatusOK, config, sh.logger)

	sh.logger.Debug(ctx, "SCIM ServiceProviderConfig GET response sent")
}

// HandleSchemaListRequest handles GET /scim/v2/Schemas.
func (sh *Handler) HandleSchemaListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	startIndex, count := scim.ParseSCIMPaginationQueryParams(r)
	listResp, svcErr := sh.svc.ListSchemas(ctx, sh.baseURL, startIndex, count)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, sh.logger)
		return
	}

	scim.WriteSCIMSuccessResponse(ctx, w, http.StatusOK, listResp, sh.logger)
	sh.logger.Debug(ctx, "SCIM Schemas list response sent",
		log.Int("totalResults", listResp.TotalResults))
}

// HandleSchemaGetRequest handles GET /scim/v2/Schemas/{id}.
func (sh *Handler) HandleSchemaGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	schemaURN := r.PathValue("id")
	if schemaURN == "" {
		scim.HandleSCIMError(w, r, &scim.ErrorSchemaNotFound, sh.logger)
		return
	}

	schema, svcErr := sh.svc.GetSchema(ctx, schemaURN, sh.baseURL)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, sh.logger)
		return
	}

	scim.WriteSCIMSuccessResponse(ctx, w, http.StatusOK, schema, sh.logger)
	sh.logger.Debug(ctx, "SCIM Schema GET response sent", log.String("urn", schemaURN))
}

// HandleResourceTypeListRequest handles GET /scim/v2/ResourceTypes.
func (sh *Handler) HandleResourceTypeListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	listResp, svcErr := sh.svc.ListResourceTypes(ctx, sh.baseURL)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, sh.logger)
		return
	}

	scim.WriteSCIMSuccessResponse(ctx, w, http.StatusOK, listResp, sh.logger)
	sh.logger.Debug(ctx, "SCIM ResourceTypes list response sent",
		log.Int("totalResults", listResp.TotalResults))
}

// HandleResourceTypeGetRequest handles GET /scim/v2/ResourceTypes/{id}.
func (sh *Handler) HandleResourceTypeGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	resourceTypeID := r.PathValue("id")
	if resourceTypeID == "" {
		scim.HandleSCIMError(w, r, &scim.ErrorResourceTypeNotFound, sh.logger)
		return
	}

	rt, svcErr := sh.svc.GetResourceType(ctx, resourceTypeID, sh.baseURL)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, sh.logger)
		return
	}

	scim.WriteSCIMSuccessResponse(ctx, w, http.StatusOK, rt, sh.logger)
	sh.logger.Debug(ctx, "SCIM ResourceType GET response sent", log.String("id", resourceTypeID))
}
