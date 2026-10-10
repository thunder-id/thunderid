// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"net/http"

	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

type appliedConfigurationHandler struct {
	service AppliedConfigurationServiceInterface
}

func newAppliedConfigurationHandler(service AppliedConfigurationServiceInterface) *appliedConfigurationHandler {
	return &appliedConfigurationHandler{service: service}
}

// handleGetAppliedConfiguration returns the configuration a gateway runs, every resource as its own
// read returns it.
func (h *appliedConfigurationHandler) handleGetAppliedConfiguration(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	configuration, svcErr := h.service.GetAppliedConfiguration(ctx, r.PathValue("id"))
	if svcErr != nil {
		handleError(ctx, w, svcErr)
		return
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, configuration)
}
