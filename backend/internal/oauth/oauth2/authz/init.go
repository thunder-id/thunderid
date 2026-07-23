/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package authz

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/par"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/revocation"
	"github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes the authorization handler and registers its routes.
func Initialize(
	mux *http.ServeMux,
	actorProvider providers.ActorProvider,
	resourceService providers.ResourceServerProvider,
	jwtService jwt.JWTServiceInterface,
	flowExecService flowexec.FlowExecServiceInterface,
	parService par.PARServiceInterface,
	criteriaRevoker revocation.CriteriaRevokerInterface,
	cfg oauthconfig.Config,
	storeProvider providers.RuntimeStoreProvider,
	transactioner transaction.Transactioner,
) (AuthorizeServiceInterface, error) {
	authzCodeStore := newAuthorizationCodeStore(storeProvider)
	authzReqStore := newAuthorizationRequestStore(storeProvider)

	authzService := newAuthorizeService(
		actorProvider, resourceService, jwtService, flowExecService,
		authzCodeStore, authzReqStore, parService, transactioner, criteriaRevoker, cfg,
	)
	authzHandler := newAuthorizeHandler(authzService, cfg)
	registerRoutes(mux, authzHandler)
	return authzService, nil
}

// registerRoutes registers the GET /oauth2/authorize route. The POST /oauth2/auth/callback
// route is registered by the callback package which dispatches by grant type.
func registerRoutes(mux *http.ServeMux, authzHandler AuthorizeHandlerInterface) {
	// CORS MUST NOT be enabled on the authorization endpoint.
	// The client redirects the user agent to it; it is not accessed directly via XHR/fetch.
	mux.HandleFunc("GET /oauth2/authorize",
		withFrameProtection(authzHandler.HandleAuthorizeGetRequest))
}

// withFrameProtection wraps an HTTP handler to prevent the page from being embedded in frames.
func withFrameProtection(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(constants.XFrameOptionsHeaderName, constants.XFrameOptionsDeny)
		w.Header().Set(constants.ContentSecurityPolicyHeaderName, constants.ContentSecurityPolicyFrameAncestorsNone)
		handler(w, r)
	}
}
