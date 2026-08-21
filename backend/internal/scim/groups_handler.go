// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package scim

import (
	"io"
	"net/http"

	scimconfig "github.com/thunder-id/thunderid/internal/scim/config"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const groupHandlerLoggerComponentName = "SCIMGroupsHandler"

// scimGroupsHandler handles SCIM HTTP requests for Groups.
type scimGroupsHandler struct {
	svc     SCIMGroupsServiceInterface
	baseURL string
}

// newSCIMGroupsHandler creates a new scimGroupsHandler instance.
func newSCIMGroupsHandler(svc SCIMGroupsServiceInterface, baseURL string) *scimGroupsHandler {
	return &scimGroupsHandler{svc: svc, baseURL: baseURL}
}

// HandleGroupsListRequest handles GET /scim/v2/Groups.
func (h *scimGroupsHandler) HandleGroupsListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, groupHandlerLoggerComponentName))
	if r.URL.Query().Get("filter") != "" {
		handleSCIMError(w, r, &ErrorFilterNotSupported)
		return
	}
	if !scimconfig.SortSupported && (r.URL.Query().Get("sortBy") != "" || r.URL.Query().Get("sortOrder") != "") {
		handleSCIMError(w, r, &ErrorSortNotSupported)
		return
	}
	startIndex, count := parseSCIMPaginationQueryParams(r)
	resp, svcErr := h.svc.ListGroups(ctx, startIndex, count, h.baseURL)
	if svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}
	writeSCIMSuccessResponse(ctx, w, http.StatusOK, resp)
	logger.Debug(ctx, "SCIM Groups list sent", log.Int("totalResults", resp.TotalResults))
}

// HandleGroupsCreateRequest handles POST /scim/v2/Groups.
func (h *scimGroupsHandler) HandleGroupsCreateRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, groupHandlerLoggerComponentName))
	if svcErr := validateSCIMContentType(r); svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		handleSCIMError(w, r, &ErrorInvalidRequestBody)
		return
	}
	payload, svcErr := parseAndValidateSCIMGroupWriteRequest(body)
	if svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}
	created, svcErr := h.svc.CreateGroup(ctx, payload.DisplayName, payload.Members, h.baseURL)
	if svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}
	w.Header().Set("Location", created.Meta.Location)
	w.Header().Set("ETag", created.Meta.Version)
	writeSCIMSuccessResponse(ctx, w, http.StatusCreated, created)
	logger.Debug(ctx, "SCIM Group created", log.String("groupID", created.ID))
}

// HandleGroupsGetRequest handles GET /scim/v2/Groups/{id}.
func (h *scimGroupsHandler) HandleGroupsGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, groupHandlerLoggerComponentName))
	groupID := r.PathValue("id")
	if groupID == "" {
		handleSCIMError(w, r, &ErrorResourceNotFound)
		return
	}
	g, svcErr := h.svc.GetGroup(ctx, groupID, h.baseURL)
	if svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}
	w.Header().Set("Location", g.Meta.Location)
	w.Header().Set("ETag", g.Meta.Version)
	writeSCIMSuccessResponse(ctx, w, http.StatusOK, g)
	logger.Debug(ctx, "SCIM Group GET sent", log.String("groupID", groupID))
}

// HandleGroupsReplaceRequest handles PUT /scim/v2/Groups/{id}.
func (h *scimGroupsHandler) HandleGroupsReplaceRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, groupHandlerLoggerComponentName))
	groupID := r.PathValue("id")
	if groupID == "" {
		handleSCIMError(w, r, &ErrorResourceNotFound)
		return
	}
	if svcErr := validateSCIMContentType(r); svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		handleSCIMError(w, r, &ErrorInvalidRequestBody)
		return
	}
	payload, svcErr := parseAndValidateSCIMGroupWriteRequest(body)
	if svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}
	replaced, svcErr := h.svc.ReplaceGroup(ctx, groupID, payload.DisplayName, payload.Members,
		r.Header.Get("If-Match"), h.baseURL)
	if svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}
	w.Header().Set("Location", replaced.Meta.Location)
	w.Header().Set("ETag", replaced.Meta.Version)
	writeSCIMSuccessResponse(ctx, w, http.StatusOK, replaced)
	logger.Debug(ctx, "SCIM Group replaced", log.String("groupID", groupID))
}

// HandleGroupsPatchRequest handles PATCH /scim/v2/Groups/{id}.
func (h *scimGroupsHandler) HandleGroupsPatchRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, groupHandlerLoggerComponentName))
	groupID := r.PathValue("id")
	if groupID == "" {
		handleSCIMError(w, r, &ErrorResourceNotFound)
		return
	}
	if svcErr := validateSCIMContentType(r); svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		handleSCIMError(w, r, &ErrorInvalidRequestBody)
		return
	}
	actions, svcErr := parseAndValidateSCIMGroupPatchRequest(body)
	if svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}
	patched, svcErr := h.svc.PatchGroup(ctx, groupID, actions, r.Header.Get("If-Match"), h.baseURL)
	if svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}
	w.Header().Set("Location", patched.Meta.Location)
	w.Header().Set("ETag", patched.Meta.Version)
	writeSCIMSuccessResponse(ctx, w, http.StatusOK, patched)
	logger.Debug(ctx, "SCIM Group patched", log.String("groupID", groupID))
}

// HandleGroupsDeleteRequest handles DELETE /scim/v2/Groups/{id}.
func (h *scimGroupsHandler) HandleGroupsDeleteRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, groupHandlerLoggerComponentName))
	groupID := r.PathValue("id")
	if groupID == "" {
		handleSCIMError(w, r, &ErrorResourceNotFound)
		return
	}
	svcErr := h.svc.DeleteGroup(ctx, groupID, r.Header.Get("If-Match"))
	if svcErr != nil {
		handleSCIMError(w, r, svcErr)
		return
	}
	writeSCIMSuccessResponse(ctx, w, http.StatusNoContent, nil)
	logger.Debug(ctx, "SCIM Group deleted", log.String("groupID", groupID))
}
