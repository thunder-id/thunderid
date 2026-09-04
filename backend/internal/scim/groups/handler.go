// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package groups implements the SCIM Groups endpoints per RFC 7643/7644.
package groups

import (
	"io"
	"net/http"

	"github.com/thunder-id/thunderid/internal/group"
	scim "github.com/thunder-id/thunderid/internal/scim/common"
	scimconfig "github.com/thunder-id/thunderid/internal/scim/config"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const groupHandlerLoggerComponentName = "SCIMGroupsHandler"

// Handler handles SCIM HTTP requests for Groups.
type Handler struct {
	svc     SCIMGroupsServiceInterface
	baseURL string
	logger  log.Logger
}

// NewHandler builds the groups service internally and returns its handler,
// the sole exported entry point the root scim package's composition root needs.
func NewHandler(groupService group.GroupServiceInterface, cfg scimconfig.SCIMConfig) *Handler {
	svc := newSCIMGroupsService(groupService)
	return &Handler{
		svc:     svc,
		baseURL: cfg.PublicURL,
		logger:  *log.GetLogger().With(log.String(log.LoggerKeyComponentName, groupHandlerLoggerComponentName)),
	}
}

// HandleGroupsListRequest handles GET /scim/v2/Groups.
func (h *Handler) HandleGroupsListRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if r.URL.Query().Get("filter") != "" {
		scim.HandleSCIMError(w, r, &scim.ErrorFilterNotSupported, groupHandlerLoggerComponentName)
		return
	}
	if !scimconfig.SortSupported && (r.URL.Query().Get("sortBy") != "" || r.URL.Query().Get("sortOrder") != "") {
		scim.HandleSCIMError(w, r, &scim.ErrorSortNotSupported, groupHandlerLoggerComponentName)
		return
	}
	startIndex, count := scim.ParseSCIMPaginationQueryParams(r)
	resp, svcErr := h.svc.ListGroups(ctx, startIndex, count, h.baseURL)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, groupHandlerLoggerComponentName)
		return
	}
	scim.WriteSCIMSuccessResponse(ctx, w, http.StatusOK, resp, groupHandlerLoggerComponentName)
	h.logger.Debug(ctx, "SCIM Groups list sent", log.Int("totalResults", resp.TotalResults))
}

// HandleGroupsCreateRequest handles POST /scim/v2/Groups.
func (h *Handler) HandleGroupsCreateRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if svcErr := scim.ValidateSCIMContentType(r); svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, groupHandlerLoggerComponentName)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, scim.MaxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		scim.HandleSCIMError(w, r, &scim.ErrorInvalidRequestBody, groupHandlerLoggerComponentName)
		return
	}
	payload, svcErr := parseAndValidateSCIMGroupWriteRequest(body)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, groupHandlerLoggerComponentName)
		return
	}
	created, svcErr := h.svc.CreateGroup(ctx, payload.DisplayName, payload.Members, h.baseURL)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, groupHandlerLoggerComponentName)
		return
	}
	w.Header().Set("Location", created.Meta.Location)
	w.Header().Set("ETag", created.Meta.Version)
	scim.WriteSCIMSuccessResponse(ctx, w, http.StatusCreated, created, groupHandlerLoggerComponentName)
	h.logger.Debug(ctx, "SCIM Group created", log.String("groupID", created.ID))
}

// HandleGroupsGetRequest handles GET /scim/v2/Groups/{id}.
func (h *Handler) HandleGroupsGetRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	groupID := r.PathValue("id")
	if groupID == "" {
		scim.HandleSCIMError(w, r, &scim.ErrorResourceNotFound, groupHandlerLoggerComponentName)
		return
	}
	g, svcErr := h.svc.GetGroup(ctx, groupID, h.baseURL)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, groupHandlerLoggerComponentName)
		return
	}
	w.Header().Set("Location", g.Meta.Location)
	w.Header().Set("ETag", g.Meta.Version)
	scim.WriteSCIMSuccessResponse(ctx, w, http.StatusOK, g, groupHandlerLoggerComponentName)
	h.logger.Debug(ctx, "SCIM Group GET sent", log.String("groupID", groupID))
}

// HandleGroupsReplaceRequest handles PUT /scim/v2/Groups/{id}.
func (h *Handler) HandleGroupsReplaceRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	groupID := r.PathValue("id")
	if groupID == "" {
		scim.HandleSCIMError(w, r, &scim.ErrorResourceNotFound, groupHandlerLoggerComponentName)
		return
	}
	if svcErr := scim.ValidateSCIMContentType(r); svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, groupHandlerLoggerComponentName)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, scim.MaxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		scim.HandleSCIMError(w, r, &scim.ErrorInvalidRequestBody, groupHandlerLoggerComponentName)
		return
	}
	payload, svcErr := parseAndValidateSCIMGroupWriteRequest(body)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, groupHandlerLoggerComponentName)
		return
	}
	replaced, svcErr := h.svc.ReplaceGroup(ctx, groupID, payload.DisplayName, payload.Members,
		r.Header.Get("If-Match"), h.baseURL)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, groupHandlerLoggerComponentName)
		return
	}
	w.Header().Set("Location", replaced.Meta.Location)
	w.Header().Set("ETag", replaced.Meta.Version)
	scim.WriteSCIMSuccessResponse(ctx, w, http.StatusOK, replaced, groupHandlerLoggerComponentName)
	h.logger.Debug(ctx, "SCIM Group replaced", log.String("groupID", groupID))
}

// HandleGroupsPatchRequest handles PATCH /scim/v2/Groups/{id}.
func (h *Handler) HandleGroupsPatchRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	groupID := r.PathValue("id")
	if groupID == "" {
		scim.HandleSCIMError(w, r, &scim.ErrorResourceNotFound, groupHandlerLoggerComponentName)
		return
	}
	if svcErr := scim.ValidateSCIMContentType(r); svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, groupHandlerLoggerComponentName)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, scim.MaxRequestBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		scim.HandleSCIMError(w, r, &scim.ErrorInvalidRequestBody, groupHandlerLoggerComponentName)
		return
	}
	actions, svcErr := parseAndValidateSCIMGroupPatchRequest(body)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, groupHandlerLoggerComponentName)
		return
	}
	patched, svcErr := h.svc.PatchGroup(ctx, groupID, actions, r.Header.Get("If-Match"), h.baseURL)
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, groupHandlerLoggerComponentName)
		return
	}
	w.Header().Set("Location", patched.Meta.Location)
	w.Header().Set("ETag", patched.Meta.Version)
	scim.WriteSCIMSuccessResponse(ctx, w, http.StatusOK, patched, groupHandlerLoggerComponentName)
	h.logger.Debug(ctx, "SCIM Group patched", log.String("groupID", groupID))
}

// HandleGroupsDeleteRequest handles DELETE /scim/v2/Groups/{id}.
func (h *Handler) HandleGroupsDeleteRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	groupID := r.PathValue("id")
	if groupID == "" {
		scim.HandleSCIMError(w, r, &scim.ErrorResourceNotFound, groupHandlerLoggerComponentName)
		return
	}
	svcErr := h.svc.DeleteGroup(ctx, groupID, r.Header.Get("If-Match"))
	if svcErr != nil {
		scim.HandleSCIMError(w, r, svcErr, groupHandlerLoggerComponentName)
		return
	}
	scim.WriteSCIMSuccessResponse(ctx, w, http.StatusNoContent, nil, groupHandlerLoggerComponentName)
	h.logger.Debug(ctx, "SCIM Group deleted", log.String("groupID", groupID))
}
