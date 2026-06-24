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

// Package oauth provides centralized initialization for all OAuth-related services.
package oauth

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/actorprovider"
	"github.com/thunder-id/thunderid/internal/attributecache"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/authz"
	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	"github.com/thunder-id/thunderid/internal/idp"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/jwks"
	oauth2authz "github.com/thunder-id/thunderid/internal/oauth/oauth2/authz"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/callback"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/ciba"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/discovery"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/granthandlers"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/introspect"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jwksresolver"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/par"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/token"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/userinfo"
	"github.com/thunder-id/thunderid/internal/oauth/scope"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/resource"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/internal/system/observability"
)

// Initialize initializes all OAuth-related services and registers their routes.
func Initialize(
	mux *http.ServeMux,
	actorProvider actorprovider.ActorProviderInterface,
	authnProvider authnprovidermgr.AuthnProviderManagerInterface,
	jwtService jwt.JWTServiceInterface,
	jweService jwe.JWEServiceInterface,
	flowExecService flowexec.FlowExecServiceInterface,
	observabilitySvc observability.ObservabilityServiceInterface,
	runtimeCrypto kmprovider.RuntimeCryptoProvider,
	ouService ou.OrganizationUnitServiceInterface,
	attributeCacheSvc attributecache.AttributeCacheServiceInterface,
	authzService authz.AuthorizationServiceInterface,
	resourceService resource.ResourceServiceInterface,
	i18nService i18nmgt.I18nServiceInterface,
	idpService idp.IDPServiceInterface,
	stores RuntimeStores,
	cfg oauthconfig.Config,
) error {
	jwks.Initialize(mux, runtimeCrypto)
	httpClient := syshttp.NewHTTPClientWithCheckRedirect(func(req *http.Request, _ []*http.Request) error {
		return syshttp.IsSSRFSafeURL(req.URL.String())
	})
	resolver := jwksresolver.Initialize(httpClient)
	tokenBuilder, tokenValidator := tokenservice.Initialize(cfg, jwtService, jweService, resolver, idpService)
	scopeValidator := scope.Initialize()
	discoveryService := discovery.Initialize(mux, runtimeCrypto, cfg)
	dpopVerifier := dpop.Initialize(cfg, stores.JTI)
	parService := par.Initialize(mux, actorProvider, authnProvider, jwtService, discoveryService,
		resourceService, dpopVerifier, stores.PAR, cfg)
	cibaService := ciba.Initialize(mux, jwtService, actorProvider, authnProvider, flowExecService,
		discoveryService, resourceService, stores.CIBA, cfg)
	oauth2AuthzService, err := oauth2authz.Initialize(mux, actorProvider, resourceService,
		jwtService, flowExecService, parService,
		stores.AuthzCode, stores.AuthzRequest, stores.AuthzTransactioner, cfg)
	if err != nil {
		return err
	}
	grantHandlerProvider := granthandlers.Initialize(
		jwtService, oauth2AuthzService, tokenBuilder, tokenValidator,
		attributeCacheSvc, ouService, authzService, actorProvider, resourceService, cibaService, cfg)
	token.Initialize(mux, jwtService, actorProvider, authnProvider, grantHandlerProvider,
		scopeValidator, observabilitySvc, discoveryService, dpopVerifier, cfg)
	introspect.Initialize(mux, jwtService, actorProvider, authnProvider, discoveryService)
	userinfo.Initialize(mux, jwtService, jweService, resolver,
		tokenValidator, actorProvider, attributeCacheSvc,
		discoveryService, dpopVerifier, cfg)
	callback.Initialize(mux, oauth2AuthzService, cibaService, cfg)
	return nil
}
