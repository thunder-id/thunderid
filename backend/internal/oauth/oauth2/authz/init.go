// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	"github.com/thunder-id/thunderid/internal/flow/session"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/par"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/revocation"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
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
	ssoSession session.Service,
	flowProvider providers.FlowProvider,
	cfg oauthconfig.Config,
	storeProvider providers.RuntimeStoreProvider,
	transactioner providers.Transactioner,
) (AuthorizeServiceInterface, error) {
	authzCodeStore := newAuthorizationCodeStore(storeProvider)
	authzReqStore := newAuthorizationRequestStore(storeProvider, cfg.OAuth.AuthorizationRequest.ValidityPeriod)

	authzService := newAuthorizeService(
		actorProvider, resourceService, jwtService, flowExecService,
		authzCodeStore, authzReqStore, parService, transactioner, criteriaRevoker,
		ssoSession, flowProvider, cfg,
	)
	authzHandler := newAuthorizeHandler(authzService, cfg)
	registerRoutes(mux, authzHandler)
	return authzService, nil
}

// registerRoutes registers the GET /oauth2/authorize route. The POST /oauth2/auth/callback
// route is registered by the callback package which dispatches by grant type.
//
// Clickjacking protection (X-Frame-Options and CSP frame-ancestors, per RFC 9700 §4.16) is applied
// globally by the security-headers middleware rather than per route, so it is not wrapped here.
func registerRoutes(mux *http.ServeMux, authzHandler AuthorizeHandlerInterface) {
	// CORS MUST NOT be enabled on the authorization endpoint.
	// The client redirects the user agent to it; it is not accessed directly via XHR/fetch.
	mux.HandleFunc("GET /oauth2/authorize", authzHandler.HandleAuthorizeGetRequest)
}
