// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package openid4vp

import (
	"context"
	"errors"
	"net/http"
	"time"

	vpprotocol "github.com/thunder-id/thunderid/internal/oauth/oauth2/openid4vp"
	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Route paths for the OpenID4VP verification session endpoints.
const (
	trustAnchorsPath = "/openid4vp/trust-anchors"
	initiatePath     = "/openid4vp/initiate"
	statusPath       = "/openid4vp/status/{txn_id}"
)

// openID4VPHandler serves the ThunderID-specific OpenID4VP verification session endpoints.
type openID4VPHandler struct {
	verifySvc vpprotocol.OpenID4VPServiceInterface
}

// newOpenID4VPHandler creates a handler backed by the given verifier service.
func newOpenID4VPHandler(verifySvc vpprotocol.OpenID4VPServiceInterface) *openID4VPHandler {
	return &openID4VPHandler{verifySvc: verifySvc}
}

// initiateRequestDTO is the request body for POST /openid4vp/initiate.
type initiateRequestDTO struct {
	DefinitionID string `json:"definition_id" native:"required"`
}

// HandleTrustAnchors returns the configured trust anchors (root CAs) as JSON.
func (h *openID4VPHandler) HandleTrustAnchors(w http.ResponseWriter, r *http.Request) {
	sysutils.WriteSuccessResponse(r.Context(), w, http.StatusOK, h.verifySvc.GetTrustAnchors())
}

// HandleInitiate handles POST /openid4vp/initiate and starts a VP verification session.
func (h *openID4VPHandler) HandleInitiate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req, err := sysutils.DecodeJSONBody[initiateRequestDTO](r)
	if err != nil {
		var valErr *sysutils.ValidationError
		if errors.As(err, &valErr) {
			sysutils.WriteStructuredErrorResponse(w, http.StatusBadRequest, "Validation Failed", valErr.Errors)
			return
		}
		sysutils.WriteErrorResponse(ctx, w, http.StatusBadRequest, apierror.ErrorResponse{
			Code:    "INVALID_REQUEST",
			Message: tidcommon.I18nMessage{DefaultValue: "Invalid request format"},
		})
		return
	}

	init, svcErr := h.verifySvc.Initiate(ctx, req.DefinitionID)
	if svcErr != nil {
		writeServiceErrorResponse(ctx, w, svcErr)
		return
	}

	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, map[string]string{
		"txn_id":     init.State,
		"wallet_url": init.WalletURI,
		"status_url": "/openid4vp/status/" + init.State,
		"expires_at": init.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

// HandleStatus handles GET /openid4vp/status/{txn_id} and polls a VP verification session.
func (h *openID4VPHandler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	txnID := sysutils.SanitizeString(r.PathValue("txn_id"))
	if txnID == "" {
		sysutils.WriteErrorResponse(ctx, w, http.StatusBadRequest, apierror.ErrorResponse{
			Code:    "INVALID_REQUEST",
			Message: tidcommon.I18nMessage{DefaultValue: "txn_id is required"},
		})
		return
	}

	rs, svcErr := h.verifySvc.GetResult(ctx, txnID)
	if svcErr != nil {
		writeServiceErrorResponse(ctx, w, svcErr)
		return
	}

	resp := map[string]string{"status": string(rs.Status)}
	if rs.ResultToken != "" {
		resp["result_token"] = rs.ResultToken
	}
	if rs.FailureReason != "" {
		resp["error"] = rs.FailureReason
	}
	sysutils.WriteSuccessResponse(ctx, w, http.StatusOK, resp)
}

// writeServiceErrorResponse maps a service error to an HTTP error response.
func writeServiceErrorResponse(ctx context.Context, w http.ResponseWriter, svcErr *tidcommon.ServiceError) {
	statusCode := http.StatusInternalServerError
	if svcErr.Type == tidcommon.ClientErrorType {
		statusCode = http.StatusBadRequest
		if svcErr.Code == vpprotocol.ErrorUnknownState.Code {
			statusCode = http.StatusNotFound
		}
	}
	sysutils.WriteErrorResponse(ctx, w, statusCode, apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	})
}
