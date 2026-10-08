// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"net/http"

	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

// versionHandler serves capturing versions and applying them to gateways.
type versionHandler struct {
	service VersionServiceInterface
}

func newVersionHandler(service VersionServiceInterface) *versionHandler {
	return &versionHandler{service: service}
}

// handleCapture records this deployment's configuration as it is now.
func (h *versionHandler) handleCapture(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req CaptureRequest
	if r.ContentLength != 0 && !decodeBody(w, r, &req) {
		return
	}
	version, svcErr := h.service.Capture(ctx, req)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	// A configuration already captured is answered with the version that holds it, not captured again.
	status := http.StatusCreated
	if version.Unchanged {
		status = http.StatusOK
	}
	sysutils.WriteSuccessResponse(ctx, w, status, version)
}

// handleListVersions lists the captured versions, newest first, without their content.
func (h *versionHandler) handleListVersions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	versions, svcErr := h.service.ListVersions(ctx)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, versions)
}

// handleGetVersion returns one version with its content.
func (h *versionHandler) handleGetVersion(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	version, svcErr := h.service.GetVersion(ctx, r.PathValue("version"))
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, version)
}

// handleGetApplied reports which version a gateway holds and which a revert would return it to.
func (h *versionHandler) handleGetApplied(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	applied, svcErr := h.service.GetApplied(ctx, r.PathValue("id"))
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, applied)
}

// handleDiff reports how a version differs from what a gateway holds.
func (h *versionHandler) handleDiff(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	diff, svcErr := h.service.Diff(ctx, r.PathValue("id"), r.URL.Query().Get("version"))
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, diff)
}

// handleApply writes a version to a gateway.
func (h *versionHandler) handleApply(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req ApplyRequest
	if r.ContentLength != 0 && !decodeBody(w, r, &req) {
		return
	}
	result, svcErr := h.service.Apply(ctx, r.PathValue("id"), req)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, result)
}

// handleRevert returns a gateway to the version it held before.
func (h *versionHandler) handleRevert(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req RevertRequest
	if r.ContentLength != 0 && !decodeBody(w, r, &req) {
		return
	}
	result, svcErr := h.service.Revert(ctx, r.PathValue("id"), req)
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, result)
}
