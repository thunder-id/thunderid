// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package deviceauth

import (
	"errors"
	"net/http"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/clientauth"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	sysconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// DeviceAuthHandlerInterface defines the Device Authorization Grant HTTP handler.
type DeviceAuthHandlerInterface interface {
	HandleDeviceAuthorizationRequest(http.ResponseWriter, *http.Request)
}

// handler serves Device Authorization Grant HTTP requests.
type handler struct {
	service DeviceAuthServiceInterface
}

// newHandler creates a Device Authorization Grant HTTP handler.
func newHandler(service DeviceAuthServiceInterface) DeviceAuthHandlerInterface {
	return &handler{service: service}
}

// HandleDeviceAuthorizationRequest starts a Device Authorization Grant request.
func (h *handler) HandleDeviceAuthorizationRequest(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		utils.WriteJSONError(
			r.Context(), w, oauth2const.ErrorInvalidRequest, "Failed to parse request body", http.StatusBadRequest, nil,
		)
		return
	}
	clientInfo := clientauth.GetOAuthClient(r.Context())
	if clientInfo == nil {
		utils.WriteJSONError(
			r.Context(), w, oauth2const.ErrorServerError, "Something went wrong", http.StatusInternalServerError, nil,
		)
		return
	}
	result, err := h.service.InitiateDeviceAuthorization(r.Context(), &DeviceAuthorizationRequest{
		Scope:     r.FormValue(oauth2const.RequestParamScope),
		Resources: r.Form[oauth2const.RequestParamResource],
	}, clientInfo.OAuthApp)
	if err != nil {
		h.writeDeviceAuthError(r, w, err)
		return
	}
	w.Header().Set(sysconst.CacheControlHeaderName, sysconst.CacheControlNoStore)
	w.Header().Set(sysconst.PragmaHeaderName, sysconst.PragmaNoCache)
	utils.WriteSuccessResponse(r.Context(), w, http.StatusOK, result)
}

// writeDeviceAuthError maps a Device Grant error to an HTTP response.
func (h *handler) writeDeviceAuthError(r *http.Request, w http.ResponseWriter, err error) {
	code, status := oauth2const.ErrorServerError, http.StatusInternalServerError
	message := "Something went wrong"
	var deviceAuthErr *DeviceAuthError
	if errors.As(err, &deviceAuthErr) {
		code = deviceAuthErr.Code
		message = deviceAuthErr.Message
		if code != oauth2const.ErrorServerError {
			status = http.StatusBadRequest
		}
	}
	utils.WriteJSONError(r.Context(), w, code, message, status, nil)
}
