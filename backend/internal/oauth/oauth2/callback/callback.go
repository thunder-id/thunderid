// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package callback owns the single POST /oauth2/auth/callback endpoint and dispatches
// completed flow assertions to the appropriate grant-type handler based on the type
// field in the request body. Adding support for a new grant type requires only a new
// case in the handler switch — no changes to the authz or ciba packages.
package callback

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	oauth2authz "github.com/thunder-id/thunderid/internal/oauth/oauth2/authz"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/ciba"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	oauth2model "github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/deviceauth"
	oauth2utils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// maxCallbackBodyBytes bounds the callback request body, which is read before the caller is checked.
const maxCallbackBodyBytes = 1 << 20 // 1 MiB

// flowCallbackRequest is the request body sent by the Gate UI to the flow callback endpoint.
// Assertion carries the terminal flow outcome, either an authentication assertion or a signed error
// assertion; the grant-type handler tells them apart from the assertion's own claims. Type identifies
// which handler processes it. When absent it defaults to authorization_code, preserving existing
// behavior for the auth code flow. SessionData is the SSO sessions the caller collected from the flow's
// responses; the authorization code keeps them when the client requested the device_sso scope.
type flowCallbackRequest struct {
	AuthID      string                    `json:"authId"`
	Assertion   string                    `json:"assertion"`
	Type        string                    `json:"type,omitempty"`
	SessionData []oauth2model.SessionData `json:"sessionData,omitempty"`
}

// callbackDispatcher dispatches flow assertion callbacks to the appropriate grant-type handler.
type callbackDispatcher struct {
	cfg           oauthconfig.Config
	authZService  oauth2authz.AuthorizeServiceInterface
	cibaService   ciba.CIBAServiceInterface
	deviceService deviceauth.DeviceAuthServiceInterface
	logger        *log.Logger
}

func newCallbackDispatcher(
	cfg oauthconfig.Config,
	authZService oauth2authz.AuthorizeServiceInterface,
	cibaService ciba.CIBAServiceInterface,
	deviceService deviceauth.DeviceAuthServiceInterface,
) *callbackDispatcher {
	return &callbackDispatcher{
		cfg:           cfg,
		authZService:  authZService,
		cibaService:   cibaService,
		deviceService: deviceService,
		logger:        log.GetLogger().With(log.String(log.LoggerKeyComponentName, "CallbackHandler")),
	}
}

// Initialize registers the flow callback route and wires the grant-type dispatcher.
func Initialize(
	mux *http.ServeMux,
	authZService oauth2authz.AuthorizeServiceInterface,
	cibaService ciba.CIBAServiceInterface,
	deviceService deviceauth.DeviceAuthServiceInterface,
	cfg oauthconfig.Config,
) {
	d := newCallbackDispatcher(cfg, authZService, cibaService, deviceService)
	corsOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST /oauth2/auth/callback", d.handleFlowCallback, corsOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /oauth2/auth/callback",
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }, corsOpts))
}

func (d *callbackDispatcher) handleFlowCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	r.Body = http.MaxBytesReader(w, r.Body, maxCallbackBodyBytes)
	req, err := utils.DecodeJSONBody[flowCallbackRequest](r)
	if err != nil {
		utils.WriteJSONError(ctx, w, oauth2const.ErrorInvalidRequest, "Invalid request body",
			http.StatusBadRequest, nil)
		return
	}

	if req.AuthID == "" {
		utils.WriteJSONError(ctx, w, oauth2const.ErrorInvalidRequest, "authId is required",
			http.StatusBadRequest, nil)
		return
	}
	if req.Assertion == "" {
		utils.WriteJSONError(ctx, w, oauth2const.ErrorInvalidRequest, "assertion is required",
			http.StatusBadRequest, nil)
		return
	}

	callbackType := req.Type
	if callbackType == "" {
		if d.deviceService != nil {
			isDeviceRequest, lookupErr := d.deviceService.IsDeviceRequest(ctx, req.AuthID)
			if lookupErr != nil {
				d.logger.Error(ctx, "Failed to determine callback grant type", log.Error(lookupErr))
				utils.WriteJSONError(ctx, w, oauth2const.ErrorServerError,
					"Failed to process authentication callback", http.StatusInternalServerError, nil)
				return
			}
			if isDeviceRequest {
				callbackType = string(providers.GrantTypeDeviceCode)
			}
		}
		if callbackType == "" {
			callbackType = string(providers.GrantTypeAuthorizationCode)
		}
	}

	switch callbackType {
	case string(providers.GrantTypeAuthorizationCode):
		redirectURI, authErr := d.authZService.HandleAuthorizationCallback(ctx, req.AuthID, req.Assertion,
			req.SessionData)
		if authErr != nil {
			if authErr.SendErrorToClient {
				d.writeRedirectWithError(ctx, w, authErr)
				return
			}
			d.writeErrorPageRedirect(ctx, w, authErr.Code, authErr.Message, authErr.State)
			return
		}
		utils.WriteSuccessResponse(ctx, w, http.StatusOK, oauth2authz.AuthZPostResponse{RedirectURI: redirectURI})

	case string(providers.GrantTypeCIBA):
		if d.cibaService == nil {
			utils.WriteJSONError(ctx, w, oauth2const.ErrorInvalidRequest,
				"Unsupported callback type", http.StatusBadRequest, nil)
			return
		}
		cibaErr := d.cibaService.HandleCallback(ctx, req.AuthID, req.Assertion)
		if cibaErr != nil {
			statusCode := http.StatusBadRequest
			if cibaErr.Code == oauth2const.ErrorServerError {
				statusCode = http.StatusInternalServerError
			}
			utils.WriteJSONError(ctx, w, cibaErr.Code, cibaErr.Message, statusCode, nil)
			return
		}
		utils.WriteSuccessResponse(ctx, w, http.StatusOK, map[string]string{"status": "OK"})

	case string(providers.GrantTypeDeviceCode):
		if d.deviceService == nil {
			utils.WriteJSONError(ctx, w, oauth2const.ErrorInvalidRequest,
				"Unsupported callback type", http.StatusBadRequest, nil)
			return
		}
		deviceErr := d.deviceService.HandleCallback(ctx, req.AuthID, req.Assertion)
		if deviceErr != nil {
			statusCode := http.StatusBadRequest
			if deviceErr.Code == oauth2const.ErrorServerError {
				statusCode = http.StatusInternalServerError
			}
			utils.WriteJSONError(ctx, w, deviceErr.Code, deviceErr.Message, statusCode, nil)
			return
		}
		utils.WriteSuccessResponse(ctx, w, http.StatusOK, map[string]string{"status": "OK"})

	default:
		utils.WriteJSONError(ctx, w, oauth2const.ErrorInvalidRequest,
			"Unsupported callback type", http.StatusBadRequest, nil)
	}
}

func (
	d *callbackDispatcher) writeRedirectWithError(ctx context.Context,
	w http.ResponseWriter,
	authErr *oauth2authz.AuthorizationError) {
	queryParams := map[string]string{
		oauth2const.RequestParamError:            authErr.Code,
		oauth2const.RequestParamErrorDescription: authErr.Message,
		oauth2const.RequestParamIss:              d.cfg.JWT.Issuer,
	}
	if authErr.State != "" {
		queryParams[oauth2const.RequestParamState] = authErr.State
	}
	redirectURI, err := oauth2utils.GetURIWithQueryParams(authErr.ClientRedirectURI, queryParams)
	if err != nil {
		d.logger.Error(ctx, "Failed to construct client redirect URI", log.Error(err))
		d.writeErrorPageRedirect(ctx, w, oauth2const.ErrorServerError,
			"Failed to process authorization request", authErr.State)
		return
	}
	utils.WriteSuccessResponse(ctx, w, http.StatusOK, oauth2authz.AuthZPostResponse{RedirectURI: redirectURI})
}

func (
	d *callbackDispatcher) writeErrorPageRedirect(ctx context.Context,
	w http.ResponseWriter,
	code,
	msg,
	state string) {
	gateClientConfig := d.cfg.GateClient
	errorPageURL := (&url.URL{
		Scheme: gateClientConfig.Scheme,
		Host:   fmt.Sprintf("%s:%d", gateClientConfig.Hostname, gateClientConfig.Port),
		Path:   gateClientConfig.ErrorPath,
	}).String()
	queryParams := map[string]string{
		"errorCode":    code,
		"errorMessage": msg,
	}
	if state != "" {
		queryParams[oauth2const.RequestParamState] = state
	}
	redirectURI, err := oauth2utils.GetURIWithQueryParams(errorPageURL, queryParams)
	if err != nil {
		http.Error(w, "Failed to redirect to error page", http.StatusInternalServerError)
		return
	}
	utils.WriteSuccessResponse(ctx, w, http.StatusOK, oauth2authz.AuthZPostResponse{RedirectURI: redirectURI})
}
