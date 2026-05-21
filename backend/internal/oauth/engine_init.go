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

package oauth

import (
	"fmt"
	"net/http"

	"github.com/thunder-id/thunderid/internal/application"
	"github.com/thunder-id/thunderid/internal/attributecache"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/authz"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	"github.com/thunder-id/thunderid/internal/oauth/jwks"
	oauthauthz "github.com/thunder-id/thunderid/internal/oauth/oauth2/authz"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dcr"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/discovery"
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
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/kmprovider"
	"github.com/thunder-id/thunderid/internal/system/observability"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

// EngineDeps holds wired dependencies for the embeddable OAuth engine runtime.
type EngineDeps struct {
	Mux *http.ServeMux

	ApplicationService application.ApplicationServiceInterface
	InboundClient      inboundclient.InboundClientServiceInterface
	AuthnProvider      authnprovidermgr.AuthnProviderManagerInterface
	JWTService         jwt.JWTServiceInterface
	JWEService         jwe.JWEServiceInterface
	FlowExecService    flowexec.FlowExecServiceInterface
	ObservabilitySvc   observability.ObservabilityServiceInterface
	RuntimeCrypto      kmprovider.RuntimeCryptoProvider
	OUService          ou.OrganizationUnitServiceInterface
	AttributeCacheSvc  attributecache.AttributeCacheServiceInterface
	AuthzService       authz.AuthorizationServiceInterface
	EntityProvider     entityprovider.EntityProviderInterface
	ResourceService    resource.ResourceServiceInterface
	I18nService        i18nmgt.I18nServiceInterface
	IDPService         idp.IDPServiceInterface

	EnableDCR bool

	PAR   *par.InitOptions
	Authz *oauthauthz.InitOptions

	// RuntimeTransactioner when set is used instead of the runtime database transactioner.
	RuntimeTransactioner transaction.Transactioner
}

// InitializeEngine initializes OAuth runtime services for the embeddable engine.
// DCR is omitted unless EnableDCR is true and ApplicationService is set.
func InitializeEngine(deps EngineDeps) error {
	mux := deps.Mux
	if mux == nil {
		return fmt.Errorf("oauth: mux is required")
	}

	transactioner := deps.RuntimeTransactioner
	if transactioner == nil {
		var err error
		transactioner, err = provider.GetDBProvider().GetRuntimeDBTransactioner()
		if err != nil {
			return err
		}
	}

	jwks.Initialize(mux, deps.RuntimeCrypto)
	httpClient := syshttp.NewHTTPClientWithCheckRedirect(func(req *http.Request, _ []*http.Request) error {
		return syshttp.IsSSRFSafeURL(req.URL.String())
	})
	resolver := jwksresolver.Initialize(httpClient)
	tokenBuilder, tokenValidator := tokenservice.Initialize(deps.JWTService, deps.JWEService, resolver, deps.IDPService)
	scopeValidator := scope.Initialize()
	discoveryService := discovery.Initialize(mux, deps.RuntimeCrypto)
	parService := par.Initialize(mux, deps.InboundClient, deps.AuthnProvider, deps.JWTService, discoveryService,
		deps.ResourceService, deps.PAR)
	grantHandlerProvider, err := granthandlers.InitializeEngine(
		mux, deps.JWTService, deps.InboundClient, deps.FlowExecService, tokenBuilder, tokenValidator,
		deps.AttributeCacheSvc, deps.OUService, deps.AuthzService, deps.EntityProvider, deps.ResourceService,
		parService, deps.Authz,
	)
	if err != nil {
		return err
	}
	token.Initialize(mux, deps.JWTService, deps.InboundClient, deps.AuthnProvider, grantHandlerProvider,
		scopeValidator, deps.ObservabilitySvc, discoveryService, transactioner)
	introspect.Initialize(mux, deps.JWTService, deps.InboundClient, deps.AuthnProvider, discoveryService)
	userinfo.Initialize(mux, deps.JWTService, deps.JWEService, resolver,
		tokenValidator, deps.InboundClient, deps.OUService, deps.AttributeCacheSvc, transactioner)
	if deps.EnableDCR && deps.ApplicationService != nil {
		dcr.Initialize(mux, deps.ApplicationService, deps.OUService, deps.I18nService, transactioner)
	}
	return nil
}
