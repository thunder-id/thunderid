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
	"net/http"

	authnAssert "github.com/thunder-id/thunderid/internal/authn/assert"
	"github.com/thunder-id/thunderid/internal/attributecache"
	flowcore "github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/executor"
	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	oauthauthz "github.com/thunder-id/thunderid/internal/oauth/oauth2/authz"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/par"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/kmprovider"
	"github.com/thunder-id/thunderid/internal/system/observability"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	"github.com/thunder-id/thunderid/pkg/thunderidengine"
)

type leanInfra struct {
	jwtService      jwt.JWTServiceInterface
	jweService      jwe.JWEServiceInterface
	runtimeCrypto   kmprovider.RuntimeCryptoProvider
	observability   observability.ObservabilityServiceInterface
	attributeCache  attributecache.AttributeCacheServiceInterface
	authAssertGen   authnAssert.AuthAssertGeneratorInterface
}

func initLeanInfra() (*leanInfra, error) {
	jwtService, jweService, runtimeCrypto, err := initCrypto()
	if err != nil {
		return nil, err
	}
	obs := observability.Initialize()
	attrCache := attributecache.Initialize()
	return &leanInfra{
		jwtService:     jwtService,
		jweService:     jweService,
		runtimeCrypto:  runtimeCrypto,
		observability:  obs,
		attributeCache: attrCache,
		authAssertGen:  authnAssert.Initialize(),
	}, nil
}

func initializeHostOnly(cfg thunderidengine.EngineConfig, mux *http.ServeMux) error {
	if err := thunderidengine.ValidateHostOnlyProviders(cfg.Providers); err != nil {
		return err
	}

	_, thunderCfg, err := loadEngineConfig(cfg.ConfigPath)
	if err != nil {
		return err
	}

	cacheManager, err := initPlatform(thunderCfg)
	if err != nil {
		return err
	}

	infra, err := initLeanInfra()
	if err != nil {
		return err
	}

	runtime, err := buildRuntimeFromHostProviders(cfg.Providers, infra)
	if err != nil {
		return err
	}

	execRegistry, err := buildHostExecutorRegistry(cfg.Executors, cfg.Providers, cacheManager, infra, runtime)
	if err != nil {
		return err
	}

	flowFactory, graphCache := flowcore.Initialize(cacheManager)
	graphBuilder := flowmgt.NewGraphBuilder(flowFactory, execRegistry, graphCache)
	runtime.flowMgt = NewRuntimeFlowDefinitionService(cfg.Providers.FlowDefinition, graphBuilder)

	if !cfg.RegisterRoutesEnabled() {
		return nil
	}
	if mux == nil {
		return fmt.Errorf("thunderidengine: mux is required when route registration is enabled")
	}

	return RegisterServerRuntime(mux, ServerRuntimeDeps{
		FlowMgtService:       runtime.flowMgt,
		InboundClient:        runtime.inbound,
		EntityProvider:       runtime.entity,
		OUService:            runtime.ou,
		DesignResolveService: runtime.designResolve,
		I18nService:          runtime.i18n,
		ExecutorRegistry:     execRegistry,
		ObservabilityService: runtime.observability,
		RuntimeCrypto:        runtime.runtimeCrypto,
		JWTService:           runtime.jwt,
		JWEService:           runtime.jwe,
		AuthnProvider:        runtime.authn,
		AuthzService:         runtime.authz,
		ResourceService:      runtime.resource,
		AttributeCache:       runtime.attributeCache,
		IDPService:           runtime.idp,
		PAR:                  runtime.parOpts,
		Authz:                runtime.authzOpts,
		FlowExec:             runtime.flowExecOpts,
		RuntimeTransactioner: transaction.NewNoOpTransactioner(),
	})
}

func buildRuntimeFromHostProviders(
	providers thunderidengine.Providers,
	infra *leanInfra,
) (*runtimeServices, error) {
	stores, err := resolveEngineRuntimeStores(providers.RuntimeStore)
	if err != nil {
		return nil, err
	}

	obs := infra.observability
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
		inbound:        newClientBridge(providers.Client),
		authn:          newAuthnBridge(providers.Authn),
		authz:          newAuthzBridge(providers.Authz),
		resource:       newResourceBridge(providers.Resource),
		ou:             newOUBridge(providers.OU),
		idp:            newIDPBridge(providers.IDP),
		entity:         newEntityBridge(providers.Client),
		observability:  obs,
		jwt:            infra.jwtService,
		jwe:            infra.jweService,
		runtimeCrypto:  infra.runtimeCrypto,
		attributeCache: infra.attributeCache,
		designResolve:  newDesignBridge(providers.Design),
		i18n:           newI18nBridge(providers.I18n),
		parOpts:        parOpts,
		authzOpts:      authzOpts,
		flowExecOpts:   flowOpts,
	}, nil
}

func buildHostExecutorRegistry(
	execCfg thunderidengine.ExecutorConfig,
	providers thunderidengine.Providers,
	cacheManager cache.CacheManagerInterface,
	infra *leanInfra,
	runtime *runtimeServices,
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

	flowFactory, _ := flowcore.Initialize(cacheManager)
	deps := executor.ExecutorDeps{
		FlowFactory:       flowFactory,
		OUService:         runtime.ou,
		IDPService:        runtime.idp,
		JWTService:        infra.jwtService,
		AuthAssertGen:     infra.authAssertGen,
		AuthnProvider:     runtime.authn,
		AuthZService:      runtime.authz,
		EntityProvider:    runtime.entity,
		AttributeCacheSvc: runtime.attributeCache,
		RoleService:       newRoleBridge(providers.Role),
	}

	names := execCfg.Names
	if len(names) == 0 {
		names = executor.DefaultExecutorNames()
	}
	reg := executor.NewExecutorRegistry()
	executor.RegisterDefaultExecutors(reg, deps, names)
	for _, ex := range execCfg.InjectCustom {
		if ex != nil {
			reg.RegisterExecutor(ex.GetName(), ex)
		}
	}
	return reg, nil
}
