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

	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	"github.com/thunder-id/thunderid/internal/flow/flowmeta"
	"github.com/thunder-id/thunderid/internal/oauth"
	"github.com/thunder-id/thunderid/pkg/thunderidengine"
)

// Initialize wires host providers from cfg into internal services and registers runtime routes on mux.
func Initialize(cfg thunderidengine.EngineConfig, mux *http.ServeMux) error {
	_, thunderCfg, err := loadEngineConfig(cfg.ConfigPath)
	if err != nil {
		return err
	}

	cacheManager, err := initPlatform(thunderCfg)
	if err != nil {
		return err
	}

	adminMux := http.NewServeMux()
	internal, err := bootstrapInternalServices(adminMux, cacheManager)
	if err != nil {
		return err
	}

	runtime, err := applyProviderOverrides(cfg.Providers, internal)
	if err != nil {
		return err
	}

	execRegistry, err := buildExecutorRegistry(cfg.Executors, internal)
	if err != nil {
		return err
	}

	runtime.flowMgt = buildFlowMgtService(cfg.Providers.FlowDefinition, cacheManager, execRegistry, internal)

	if !cfg.RegisterRoutesEnabled() {
		return nil
	}

	routeMux := mux
	if routeMux == nil {
		routeMux = http.NewServeMux()
	}

	flowExecService, err := flowexec.Initialize(routeMux, runtime.flowMgt, runtime.inbound, runtime.entity,
		execRegistry, runtime.observability, runtime.runtimeCrypto, runtime.flowExecOpts)
	if err != nil {
		return err
	}

	flowmeta.Initialize(routeMux, runtime.inbound, runtime.entity, runtime.ou,
		runtime.designResolve, runtime.i18n)

	return oauth.InitializeEngine(oauth.EngineDeps{
		Mux:               routeMux,
		InboundClient:     runtime.inbound,
		AuthnProvider:     runtime.authn,
		JWTService:        runtime.jwt,
		JWEService:        runtime.jwe,
		FlowExecService:   flowExecService,
		ObservabilitySvc:  runtime.observability,
		RuntimeCrypto:     runtime.runtimeCrypto,
		OUService:         runtime.ou,
		AttributeCacheSvc: runtime.attributeCache,
		AuthzService:      runtime.authz,
		EntityProvider:    runtime.entity,
		ResourceService:   runtime.resource,
		IDPService:        runtime.idp,
		EnableDCR:         false,
		PAR:               runtime.parOpts,
		Authz:             runtime.authzOpts,
	})
}
