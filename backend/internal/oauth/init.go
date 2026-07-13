/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

	"github.com/thunder-id/thunderid/internal/attributecache"
	"github.com/thunder-id/thunderid/internal/flow/flowexec"
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
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/revocation"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/token"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/userinfo"
	"github.com/thunder-id/thunderid/internal/oauth/scope"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	kmprovider "github.com/thunder-id/thunderid/internal/system/kmprovider/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Initialize initializes all OAuth-related services and registers their routes.
func Initialize(
	mux *http.ServeMux,
	actorProvider providers.ActorProvider,
	authnProvider providers.AuthnProviderManager,
	jwtService jwt.JWTServiceInterface,
	jweService jwe.JWEServiceInterface,
	flowExecService flowexec.FlowExecServiceInterface,
	observabilitySvc providers.ObservabilityProvider,
	runtimeCrypto kmprovider.RuntimeCryptoProvider,
	ouService providers.OrganizationUnitProvider,
	attributeCacheSvc attributecache.AttributeCacheServiceInterface,
	authzService providers.AuthorizationProvider,
	resourceService providers.ResourceServerProvider,
	i18nService providers.I18nProvider,
	idpService providers.IDPProvider,
	dpopVerifier dpop.VerifierInterface,
	cfg oauthconfig.Config,
) error {
	jwks.Initialize(mux, runtimeCrypto)
	httpClient := syshttp.NewHTTPClientWithCheckRedirect(func(req *http.Request, _ []*http.Request) error {
		return syshttp.IsSSRFSafeURL(req.URL.String())
	})
	resolver := jwksresolver.Initialize(httpClient)
	scopeValidator := scope.Initialize()
	discoveryService := discovery.Initialize(mux, runtimeCrypto, cfg)
	// The enforcement service (revocation read path) is built before the token service so it can be
	// injected into the validator, which enforces the deny list as the final step of every validation.
	enforcementService, refreshTokenRevoker := revocation.Initialize(
		mux, jwtService, actorProvider, authnProvider, discoveryService, observabilitySvc)
	tokenBuilder, tokenValidator := tokenservice.Initialize(
		cfg, jwtService, jweService, resolver, idpService, enforcementService)
	parService := par.Initialize(mux, actorProvider, authnProvider, jwtService, discoveryService,
		resourceService, dpopVerifier, cfg)
	cibaService := ciba.Initialize(mux, jwtService, actorProvider, authnProvider, flowExecService,
		discoveryService, resourceService, cfg)
	oauth2AuthzService, err := oauth2authz.Initialize(mux, actorProvider, resourceService,
		jwtService, flowExecService, parService, cfg)
	if err != nil {
		return err
	}
	grantHandlerProvider := granthandlers.Initialize(
		jwtService, oauth2AuthzService, tokenBuilder, tokenValidator,
		attributeCacheSvc, ouService, authzService, actorProvider, resourceService, cibaService,
		refreshTokenRevoker, cfg)
	token.Initialize(mux, jwtService, actorProvider, authnProvider, grantHandlerProvider,
		scopeValidator, observabilitySvc, discoveryService, dpopVerifier, cfg)
	introspect.Initialize(mux, jwtService, actorProvider, authnProvider, discoveryService, tokenValidator)
	userinfo.Initialize(mux, jwtService, jweService, resolver,
		tokenValidator, actorProvider, attributeCacheSvc,
		discoveryService, dpopVerifier, cfg)
	callback.Initialize(mux, oauth2AuthzService, cibaService, cfg)
	return nil
}
