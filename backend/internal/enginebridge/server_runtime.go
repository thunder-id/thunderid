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

package enginebridge

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/application"
	"github.com/thunder-id/thunderid/internal/attributecache"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/authz"
	"github.com/thunder-id/thunderid/internal/design/resolve"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/executor"
	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	"github.com/thunder-id/thunderid/internal/flow/flowmeta"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	"github.com/thunder-id/thunderid/internal/oauth"
	oauthauthz "github.com/thunder-id/thunderid/internal/oauth/oauth2/authz"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/par"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/resource"
	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/kmprovider"
	"github.com/thunder-id/thunderid/internal/system/observability"
)

// ServerRuntimeDeps holds Thunder services already initialized by cmd/server.
// RegisterServerRuntime wires the same runtime routes as thunderidengine.Engine.Initialize
// without re-bootstrapping domain services.
type ServerRuntimeDeps struct {
	FlowMgtService       flowmgt.FlowMgtServiceInterface
	InboundClient        inboundclient.InboundClientServiceInterface
	EntityProvider       entityprovider.EntityProviderInterface
	OUService            ou.OrganizationUnitServiceInterface
	DesignResolveService resolve.DesignResolveServiceInterface
	I18nService          i18nmgt.I18nServiceInterface
	ExecutorRegistry     executor.ExecutorRegistryInterface
	ObservabilityService observability.ObservabilityServiceInterface
	RuntimeCrypto        kmprovider.RuntimeCryptoProvider
	JWTService           jwt.JWTServiceInterface
	JWEService           jwe.JWEServiceInterface
	AuthnProvider        authnprovidermgr.AuthnProviderManagerInterface
	AuthzService         authz.AuthorizationServiceInterface
	ResourceService      resource.ResourceServiceInterface
	AttributeCache       attributecache.AttributeCacheServiceInterface
	IDPService           idp.IDPServiceInterface
	ApplicationService   application.ApplicationServiceInterface
	EnableDCR            bool
	PAR                  *par.InitOptions
	Authz                *oauthauthz.InitOptions
	FlowExec             *flowexec.InitOptions
}

// RegisterServerRuntime registers POST /flow/execute, GET /flow/meta, and OAuth AS routes (optional DCR).
func RegisterServerRuntime(mux *http.ServeMux, deps ServerRuntimeDeps) error {
	flowExecService, err := flowexec.Initialize(
		mux, deps.FlowMgtService, deps.InboundClient, deps.EntityProvider,
		deps.ExecutorRegistry, deps.ObservabilityService, deps.RuntimeCrypto, deps.FlowExec,
	)
	if err != nil {
		return err
	}

	flowmeta.Initialize(mux, deps.InboundClient, deps.EntityProvider, deps.OUService,
		deps.DesignResolveService, deps.I18nService)

	return oauth.InitializeEngine(oauth.EngineDeps{
		Mux:                mux,
		ApplicationService: deps.ApplicationService,
		InboundClient:      deps.InboundClient,
		AuthnProvider:      deps.AuthnProvider,
		JWTService:         deps.JWTService,
		JWEService:         deps.JWEService,
		FlowExecService:    flowExecService,
		ObservabilitySvc:   deps.ObservabilityService,
		RuntimeCrypto:      deps.RuntimeCrypto,
		OUService:          deps.OUService,
		AttributeCacheSvc:  deps.AttributeCache,
		AuthzService:       deps.AuthzService,
		EntityProvider:     deps.EntityProvider,
		ResourceService:    deps.ResourceService,
		I18nService:        deps.I18nService,
		IDPService:         deps.IDPService,
		EnableDCR:          deps.EnableDCR,
		PAR:                deps.PAR,
		Authz:              deps.Authz,
	})
}
