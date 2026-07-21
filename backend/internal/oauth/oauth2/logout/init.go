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
	"net/http"

	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize wires the RP-initiated logout feature and registers the end_session_endpoint.
func Initialize(
	mux *http.ServeMux,
	jwtService jwt.JWTServiceInterface,
	actorProvider providers.ActorProvider,
	flowExecService flowexec.FlowExecServiceInterface,
	runtimeStore providers.RuntimeStoreProvider,
	cfg oauthconfig.Config,
) {
	store := newLogoutRequestStore(runtimeStore)
	service := newLogoutService(jwtService, actorProvider, flowExecService, store, cfg.JWT.Issuer)
	handler := newLogoutHandler(service, cfg)
	registerRoutes(mux, handler)
}

// registerRoutes registers the GET/POST/OPTIONS routes for the logout endpoint and its completion
// callback (POST /oauth2/logout/callback), which the gate calls once the sign-out flow finishes.
func registerRoutes(mux *http.ServeMux, handler *logoutHandler) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	callbackEndpoint := constants.OAuth2LogoutEndpoint + "/callback"

	mux.HandleFunc(middleware.WithCORS("GET "+constants.OAuth2LogoutEndpoint, handler.HandleLogout, opts))
	mux.HandleFunc(middleware.WithCORS("POST "+constants.OAuth2LogoutEndpoint, handler.HandleLogout, opts))
	mux.HandleFunc(middleware.WithCORS("POST "+callbackEndpoint, handler.HandleLogoutCallback, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+constants.OAuth2LogoutEndpoint,
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS "+callbackEndpoint,
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
}
