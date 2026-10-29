// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/middleware"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// handleGetMaster reports what a gateway holds, resource by resource.
func (h *versionHandler) handleGetMaster(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	master, svcErr := h.service.GetMaster(ctx, r.PathValue("id"))
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, master)
}

// handleListMasters lists a gateway's masters, newest first.
func (h *versionHandler) handleListMasters(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	masters, svcErr := h.service.ListMasters(ctx, r.PathValue("id"))
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, masters)
}

// handleGetResourceHistory reports one resource's entry across a gateway's masters. The resource is
// named by its key, type and id, as the diff and the master name it.
func (h *versionHandler) handleGetResourceHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	key := r.PathValue("type") + "/" + r.PathValue("resource")
	history, svcErr := h.service.GetResourceHistory(ctx, r.PathValue("id"), key)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, history)
}

// registerMasterRoutes serves what each gateway holds.
func registerMasterRoutes(mux *http.ServeMux, h *versionHandler) {
	noContent := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
	readOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	for path, handler := range map[string]http.HandlerFunc{
		"/gateways/{id}/master":                             h.handleGetMaster,
		"/gateways/{id}/master/history":                     h.handleListMasters,
		"/gateways/{id}/master/resources/{type}/{resource}": h.handleGetResourceHistory,
	} {
		mux.HandleFunc(middleware.WithCORS("GET "+path, handler, readOpts))
		mux.HandleFunc(middleware.WithCORS("OPTIONS "+path, noContent, readOpts))
	}
}
