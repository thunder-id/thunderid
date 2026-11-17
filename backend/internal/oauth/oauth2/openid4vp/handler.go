// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package openid4vp

import (
	"context"
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/error/apierror"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

const requestObjectContentType = "application/oauth-authz-req+jwt"

// Route paths for the OpenID4VP endpoints.
const (
	requestURIPath  = "/openid4vp/request"
	responseURIPath = "/openid4vp/response"
)

// openID4VPHandler serves the OpenID4VP endpoints.
type openID4VPHandler struct {
	walletSvc walletInterface
	logger    *log.Logger
}

// newOpenID4VPHandler creates a handler wired with the given service implementations.
func newOpenID4VPHandler(walletSvc walletInterface) *openID4VPHandler {
	return &openID4VPHandler{
		walletSvc: walletSvc,
		logger:    log.GetLogger().With(log.String(log.LoggerKeyComponentName, "OpenID4VPHandler")),
	}
}

// HandleRequestObject returns the signed authorization request JWT to the wallet.
func (h *openID4VPHandler) HandleRequestObject(w http.ResponseWriter, r *http.Request) {
	state := sysutils.SanitizeString(r.URL.Query().Get("state"))
	if state == "" {
		writeServiceErrorResponse(r.Context(), w, &ErrorInvalidRequest)
		return
	}

	jar, svcErr := h.walletSvc.GetRequestObject(r.Context(), state)
	if svcErr != nil {
		writeServiceErrorResponse(r.Context(), w, svcErr)
		return
	}

	w.Header().Set("Content-Type", requestObjectContentType)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if _, werr := w.Write([]byte(jar)); werr != nil {
		h.logger.Error(r.Context(), "Failed to write request object response", log.Error(werr))
	}
}

// HandleResponse ingests the wallet's encrypted VP response.
func (h *openID4VPHandler) HandleResponse(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeServiceErrorResponse(r.Context(), w, &ErrorInvalidRequest)
		return
	}

	state := sysutils.SanitizeString(r.FormValue("state"))
	if state == "" {
		writeServiceErrorResponse(r.Context(), w, &ErrorInvalidRequest)
		return
	}

	// A wallet may post an OAuth error to the response_uri instead of a vp_token (OpenID4VP §6.4).
	if errCode := sysutils.SanitizeString(r.FormValue("error")); errCode != "" {
		if svcErr := h.walletSvc.SubmitError(r.Context(), state, errCode,
			r.FormValue("error_description")); svcErr != nil {
			writeServiceErrorResponse(r.Context(), w, svcErr)
			return
		}
		sysutils.WriteSuccessResponse(r.Context(), w, http.StatusOK, map[string]string{})
		return
	}

	response := r.FormValue("response")
	if response == "" {
		writeServiceErrorResponse(r.Context(), w, &ErrorInvalidRequest)
		return
	}

	_, redirect, svcErr := h.walletSvc.SubmitResponse(r.Context(), state, []byte(response))
	if svcErr != nil {
		writeServiceErrorResponse(r.Context(), w, svcErr)
		return
	}

	body := map[string]string{}
	if redirect != "" {
		body["redirect_uri"] = redirect
	}
	sysutils.WriteSuccessResponse(r.Context(), w, http.StatusOK, body)
}

// writeServiceErrorResponse maps a service error to an HTTP error response.
func writeServiceErrorResponse(ctx context.Context, w http.ResponseWriter, svcErr *tidcommon.ServiceError) {
	statusCode := http.StatusInternalServerError
	if svcErr.Type == tidcommon.ClientErrorType {
		statusCode = http.StatusBadRequest
		if svcErr.Code == ErrorUnknownState.Code {
			statusCode = http.StatusNotFound
		}
	}
	sysutils.WriteErrorResponse(ctx, w, statusCode, apierror.ErrorResponse{
		Code:        svcErr.Code,
		Message:     svcErr.Error,
		Description: svcErr.ErrorDescription,
	})
}
