// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package scim

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/log"
)

const handlerLoggerComponentName = "SCIMHandler"

// scimDiscoveryHandler handles SCIM discovery HTTP requests.
type scimDiscoveryHandler struct {
	svc     SCIMDiscoveryServiceInterface
	baseURL string
}

// newSCIMDiscoveryHandler creates a new scimDiscoveryHandler instance.
func newSCIMDiscoveryHandler(svc SCIMDiscoveryServiceInterface, baseURL string) *scimDiscoveryHandler {
	return &scimDiscoveryHandler{
		svc:     svc,
		baseURL: baseURL,
	}
}

// HandleServiceProviderConfigGetRequest handles GET /scim/v2/ServiceProviderConfig.
func (sh *scimDiscoveryHandler) HandleServiceProviderConfigGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	config := sh.svc.GetServiceProviderConfig(ctx, sh.baseURL)
	writeSCIMSuccessResponse(ctx, w, http.StatusOK, config)

	logger.Debug(ctx, "SCIM ServiceProviderConfig GET response sent")
}

// HandleSchemaListRequest handles GET /scim/v2/Schemas.
// Returns all SCIM schemas: the core User schema plus one per ThunderID user type.
func (sh *scimDiscoveryHandler) HandleSchemaListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	listResp, svcErr := sh.svc.ListSchemas(ctx, sh.baseURL)
	if svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}

	writeSCIMSuccessResponse(ctx, w, http.StatusOK, listResp)
	logger.Debug(ctx, "SCIM Schemas list response sent",
		log.Int("totalResults", listResp.TotalResults))
}

// HandleSchemaGetRequest handles GET /scim/v2/Schemas/{id}.
// The {id} path value is the full SCIM schema URN (e.g.
// urn:ietf:params:scim:schemas:core:2.0:User or a ThunderID extension URN).
func (sh *scimDiscoveryHandler) HandleSchemaGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))
	schemaURN := r.PathValue("id")
	if schemaURN == "" {
		handleSCIMError(w, r, &ErrorSchemaNotFound)
		return
	}

	schema, svcErr := sh.svc.GetSchema(ctx, schemaURN, sh.baseURL)
	if svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}

	writeSCIMSuccessResponse(ctx, w, http.StatusOK, schema)
	logger.Debug(ctx, "SCIM Schema GET response sent", log.String("urn", schemaURN))
}

// HandleResourceTypeListRequest handles GET /scim/v2/ResourceTypes.
// Returns all SCIM resource types. ThunderID only exposes a single "User" resource type.
func (sh *scimDiscoveryHandler) HandleResourceTypeListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))
	listResp, svcErr := sh.svc.ListResourceTypes(ctx, sh.baseURL)
	if svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}

	writeSCIMSuccessResponse(ctx, w, http.StatusOK, listResp)
	logger.Debug(ctx, "SCIM ResourceTypes list response sent",
		log.Int("totalResults", listResp.TotalResults))
}

// HandleResourceTypeGetRequest handles GET /scim/v2/ResourceTypes/{id}.
// The {id} path value is the resource type name — "User" is the only supported value.
func (sh *scimDiscoveryHandler) HandleResourceTypeGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, handlerLoggerComponentName))

	resourceTypeID := r.PathValue("id")
	if resourceTypeID == "" {
		handleSCIMError(w, r, &ErrorResourceTypeNotFound)
		return
	}

	rt, svcErr := sh.svc.GetResourceType(ctx, resourceTypeID, sh.baseURL)
	if svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}

	writeSCIMSuccessResponse(ctx, w, http.StatusOK, rt)
	logger.Debug(ctx, "SCIM ResourceType GET response sent", log.String("id", resourceTypeID))
}

// handleUnsupportedRequest handles unimplemented endpoints by returning a SCIM-standard 501.
// Delegates to handleSCIMError so that all error paths go through the same translator.
func (sh *scimDiscoveryHandler) handleUnsupportedRequest(w http.ResponseWriter, r *http.Request) {
	handleSCIMError(w, r, &ErrorUnsupportedOperation)
}
