/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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

	oauth2authz "github.com/thunder-id/thunderid/internal/oauth/oauth2/authz"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/ciba"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	oauth2utils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// flowCallbackRequest is the request body sent by the Gate UI to the flow callback endpoint.
// Type identifies which grant-type handler processes the completed assertion. When absent it
// defaults to authorization_code, preserving existing behavior for the auth code flow.
type flowCallbackRequest struct {
	AuthID    string `json:"authId"`
	Assertion string `json:"assertion"`
	Type      string `json:"type,omitempty"`
}

// callbackDispatcher dispatches flow assertion callbacks to the appropriate grant-type handler.
type callbackDispatcher struct {
	authZService oauth2authz.AuthorizeServiceInterface
	cibaService  ciba.CIBAServiceInterface
	logger       *log.Logger
}

func newCallbackDispatcher(
	authZService oauth2authz.AuthorizeServiceInterface,
	cibaService ciba.CIBAServiceInterface,
) *callbackDispatcher {
	return &callbackDispatcher{
		authZService: authZService,
		cibaService:  cibaService,
		logger:       log.GetLogger().With(log.String(log.LoggerKeyComponentName, "CallbackHandler")),
	}
}

// Initialize registers the flow callback route and wires the grant-type dispatcher.
func Initialize(
	mux *http.ServeMux,
	authZService oauth2authz.AuthorizeServiceInterface,
	cibaService ciba.CIBAServiceInterface,
) {
	d := newCallbackDispatcher(authZService, cibaService)
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

	req, err := utils.DecodeJSONBody[flowCallbackRequest](r)
	if err != nil {
		utils.WriteJSONError(ctx, w, oauth2const.ErrorInvalidRequest, "Invalid request body",
			http.StatusBadRequest, nil)
		return
	}

	if req.AuthID == "" || req.Assertion == "" {
		utils.WriteJSONError(ctx, w, oauth2const.ErrorInvalidRequest, "authId and assertion are required",
			http.StatusBadRequest, nil)
		return
	}

	callbackType := req.Type
	if callbackType == "" {
		callbackType = string(oauth2const.GrantTypeAuthorizationCode)
	}

	switch callbackType {
	case string(oauth2const.GrantTypeAuthorizationCode):
		redirectURI, authErr := d.authZService.HandleAuthorizationCallback(ctx, req.AuthID, req.Assertion)
		if authErr != nil {
			if authErr.SendErrorToClient {
				d.writeRedirectWithError(ctx, w, authErr)
				return
			}
			d.writeErrorPageRedirect(ctx, w, authErr.Code, authErr.Message, authErr.State)
			return
		}
		utils.WriteSuccessResponse(ctx, w, http.StatusOK, oauth2authz.AuthZPostResponse{RedirectURI: redirectURI})

	case string(oauth2const.GrantTypeCIBA):
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
		oauth2const.RequestParamIss:              config.GetServerRuntime().Config.JWT.Issuer,
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
	gateClientConfig := config.GetServerRuntime().Config.GateClient
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
