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

package logout

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	oauth2utils "github.com/thunder-id/thunderid/internal/oauth/oauth2/utils"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// paramLogoutID is the gate query/callback parameter carrying the stored logout-request id.
const paramLogoutID = "logoutId"

// logoutHandler serves the RP-initiated logout endpoint. It validates the request, persists the
// validated post-logout target server-side, initiates the application's sign-out flow, and redirects the
// browser to the gate sign-out page to run the flow (confirmation + session termination). The gate
// executes the flow via /flow/execute (which clears the per-flow cookie), then calls back to the
// completion endpoint, which issues the post-logout redirect. Keeping the post-logout target in the
// OAuth layer (not the flow) leaves the flow engine protocol-agnostic and gives OAuth a hook for
// protocol-level actions on sign-out.
type logoutHandler struct {
	service    LogoutServiceInterface
	gateConfig oauthconfig.Config
	logger     *log.Logger
}

func newLogoutHandler(service LogoutServiceInterface, gateConfig oauthconfig.Config) *logoutHandler {
	return &logoutHandler{
		service:    service,
		gateConfig: gateConfig,
		logger:     log.GetLogger().With(log.String(log.LoggerKeyComponentName, "LogoutHandler")),
	}
}

// HandleLogout handles GET and POST requests to the end_session_endpoint.
func (h *logoutHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	req := LogoutRequest{
		IDTokenHint:           r.FormValue(constants.RequestParamIDTokenHint),
		ClientID:              r.FormValue(constants.RequestParamClientID),
		PostLogoutRedirectURI: r.FormValue(constants.RequestParamPostLogoutRedirect),
		State:                 r.FormValue(constants.RequestParamState),
		Headers:               sysutils.SanitizeRawMultiValueStringMap(r.Header),
		QueryParams:           sysutils.SanitizeRawMultiValueStringMap(r.URL.Query()),
	}

	// Validate before initiating anything: the post-logout redirect URI is validated here (against the
	// client's registered list) and never trusted from the browser again.
	resolution, err := h.service.Resolve(r.Context(), req)
	if err != nil {
		h.logger.Debug(r.Context(), "Rejected logout request", log.Error(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	initiation, svcErr := h.service.InitiateSignOutFlow(r.Context(), resolution)
	if svcErr != nil {
		status := http.StatusInternalServerError
		if svcErr.Type == tidcommon.ClientErrorType {
			// Client errors are the caller's fault; log at debug, not error.
			status = http.StatusBadRequest
			h.logger.Debug(r.Context(), "Rejected sign-out flow initiation",
				log.String("appID", resolution.AppID), log.String("error", svcErr.Error.DefaultValue))
		} else {
			h.logger.Error(r.Context(), "Failed to initiate sign-out flow",
				log.String("appID", resolution.AppID), log.String("error", svcErr.Error.DefaultValue))
		}
		http.Error(w, "logout failed", status)
		return
	}

	redirectURL, buildErr := getSignOutPageRedirectURI(h.gateConfig, map[string]string{
		constants.AppID:       resolution.AppID,
		constants.ExecutionID: initiation.ExecutionID,
		paramLogoutID:         initiation.LogoutID,
	})
	if buildErr != nil {
		h.logger.Error(r.Context(), "Failed to build gate sign-out redirect", log.Error(buildErr))
		http.Error(w, "logout failed", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// HandleLogoutCallback completes an RP-initiated sign-out. The gate posts the logout id here once the
// sign-out flow finishes; the server consumes the stored request (running any protocol-level actions)
// and returns the post-logout redirect URI for the browser to land on.
func (h *logoutHandler) HandleLogoutCallback(w http.ResponseWriter, r *http.Request) {
	var body struct {
		LogoutID string `json:"logoutId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.LogoutID == "" {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	redirectURI, err := h.service.CompleteSignOut(r.Context(), body.LogoutID)
	if err != nil {
		h.logger.Error(r.Context(), "Failed to complete sign-out", log.Error(err))
		http.Error(w, "logout failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set(serverconst.ContentTypeHeaderName, serverconst.ContentTypeJSON)
	if encErr := json.NewEncoder(w).Encode(map[string]string{"redirect_uri": redirectURI}); encErr != nil {
		h.logger.Error(r.Context(), "Failed to encode sign-out callback response", log.Error(encErr))
	}
}

// getSignOutPageRedirectURI builds the gate sign-out page URL with the given query params.
func getSignOutPageRedirectURI(cfg oauthconfig.Config, queryParams map[string]string) (string, error) {
	signOutPageURL := (&url.URL{
		Scheme: cfg.GateClient.Scheme,
		Host:   fmt.Sprintf("%s:%d", cfg.GateClient.Hostname, cfg.GateClient.Port),
		Path:   cfg.GateClient.SignOutPath,
	}).String()

	return oauth2utils.GetURIWithQueryParams(signOutPageURL, queryParams)
}
