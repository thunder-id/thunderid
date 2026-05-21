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
	"fmt"

	"github.com/thunder-id/thunderid/internal/attributecache"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/authz"
	"github.com/thunder-id/thunderid/internal/design/resolve"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	flowcore "github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/executor"
	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	oauthauthz "github.com/thunder-id/thunderid/internal/oauth/oauth2/authz"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/par"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/system/cache"
	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/kmprovider"
	"github.com/thunder-id/thunderid/internal/system/observability"
	"github.com/thunder-id/thunderid/pkg/thunderidengine"
)

type runtimeServices struct {
	inbound        inboundclient.InboundClientServiceInterface
	authn          authnprovidermgr.AuthnProviderManagerInterface
	authz          authz.AuthorizationServiceInterface
	resource       resource.ResourceServiceInterface
	ou             ou.OrganizationUnitServiceInterface
	idp            idp.IDPServiceInterface
	entity         entityprovider.EntityProviderInterface
	flowMgt        flowmgt.FlowMgtServiceInterface
	observability  observability.ObservabilityServiceInterface
	jwt            jwt.JWTServiceInterface
	jwe            jwe.JWEServiceInterface
	runtimeCrypto  kmprovider.RuntimeCryptoProvider
	attributeCache attributecache.AttributeCacheServiceInterface
	designResolve  resolve.DesignResolveServiceInterface
	i18n           i18nmgt.I18nServiceInterface
	parOpts        *par.InitOptions
	authzOpts      *oauthauthz.InitOptions
	flowExecOpts   *flowexec.InitOptions
}

func applyProviderOverrides(
	providers thunderidengine.Providers,
	internal *internalServices,
) (*runtimeServices, error) {
	stores, err := resolveEngineRuntimeStores(providers.RuntimeStore)
	if err != nil {
		return nil, err
	}

	inbound := internal.InboundClient
	if providers.Client != nil {
		inbound = newClientBridge(providers.Client)
	}
	authn := internal.AuthnProvider
	if providers.Authn != nil {
		authn = newAuthnBridge(providers.Authn)
	}
	authzSvc := internal.AuthzService
	if providers.Authz != nil {
		authzSvc = newAuthzBridge(providers.Authz)
	}
	resourceSvc := internal.ResourceService
	if providers.Resource != nil {
		resourceSvc = newResourceBridge(providers.Resource)
	}
	ouSvc := internal.OUService
	if providers.OU != nil {
		ouSvc = newOUBridge(providers.OU)
	}
	idpSvc := internal.IDPService
	if providers.IDP != nil {
		idpSvc = newIDPBridge(providers.IDP)
	}
	entity := internal.EntityProvider
	if providers.Client != nil {
		entity = newEntityBridge(providers.Client)
	}
	obs := internal.Observability
	if providers.Observability != nil {
		obs = newObservabilityBridge(providers.Observability)
	}

	var parOpts *par.InitOptions
	var authzOpts *oauthauthz.InitOptions
	var flowOpts *flowexec.InitOptions
	if stores.PAR != nil {
		parOpts = &par.InitOptions{Store: stores.PAR}
	}
	if stores.AuthCode != nil || stores.AuthRequest != nil {
		authzOpts = &oauthauthz.InitOptions{
			CodeStore:    stores.AuthCode,
			RequestStore: stores.AuthRequest,
		}
	}
	if stores.FlowContext != nil {
		flowOpts = &flowexec.InitOptions{ContextStore: stores.FlowContext}
	}

	return &runtimeServices{
		inbound:        inbound,
		authn:          authn,
		authz:          authzSvc,
		resource:       resourceSvc,
		ou:             ouSvc,
		idp:            idpSvc,
		entity:         entity,
		flowMgt:        internal.FlowMgtService,
		observability:  obs,
		jwt:            internal.JWTService,
		jwe:            internal.JWEService,
		runtimeCrypto:  internal.RuntimeCrypto,
		attributeCache: internal.AttributeCache,
		designResolve:  internal.DesignResolve,
		i18n:           internal.I18nService,
		parOpts:        parOpts,
		authzOpts:      authzOpts,
		flowExecOpts:   flowOpts,
	}, nil
}

func resolveEngineRuntimeStores(host thunderidengine.RuntimeStore) (RuntimeStores, error) {
	if host != nil {
		return NewHostRuntimeStores(host), nil
	}
	defaultStore, err := NewDefaultRuntimeStore()
	if err != nil {
		return RuntimeStores{}, err
	}
	return NewHostRuntimeStores(defaultStore), nil
}

func buildExecutorRegistry(
	execCfg thunderidengine.ExecutorConfig,
	internal *internalServices,
) (executor.ExecutorRegistryInterface, error) {
	if execCfg.CustomRegistry != nil {
		reg := newExecutorRegistryBridge(execCfg.CustomRegistry)
		for _, ex := range execCfg.InjectCustom {
			if ex != nil {
				reg.RegisterExecutor(ex.GetName(), ex)
			}
		}
		return reg, nil
	}
	if len(execCfg.Names) == 0 && len(execCfg.InjectCustom) == 0 {
		return internal.ExecRegistry, nil
	}
	names := execCfg.Names
	if len(names) == 0 {
		names = executor.DefaultExecutorNames()
	}
	reg := executor.NewExecutorRegistry()
	for _, name := range names {
		ex, err := internal.ExecRegistry.GetExecutor(name)
		if err != nil {
			return nil, fmt.Errorf("thunderidengine: executor %q: %w", name, err)
		}
		reg.RegisterExecutor(name, ex)
	}
	for _, ex := range execCfg.InjectCustom {
		if ex != nil {
			reg.RegisterExecutor(ex.GetName(), ex)
		}
	}
	return reg, nil
}

func buildFlowMgtService(
	flowDef thunderidengine.FlowDefinitionProvider,
	cacheManager cache.CacheManagerInterface,
	execRegistry executor.ExecutorRegistryInterface,
	internal *internalServices,
) flowmgt.FlowMgtServiceInterface {
	if flowDef == nil {
		return internal.FlowMgtService
	}
	flowFactory, graphCache := flowcore.Initialize(cacheManager)
	graphBuilder := flowmgt.NewGraphBuilder(flowFactory, execRegistry, graphCache)
	return NewRuntimeFlowDefinitionService(flowDef, graphBuilder)
}
