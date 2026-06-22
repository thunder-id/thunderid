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

package thunderidengine

import (
	"fmt"
	"net/http"

	flowcore "github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/executor"
	"github.com/thunder-id/thunderid/internal/flow/interceptor"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
)

// buildDefaultFlowProvider constructs a read-only, file-backed flow provider from the
// configured declarative flow resources, for use when WithFlowProvider was not supplied and
// declarative mode is enabled. The /flows management routes that flow management registers are
// mounted on a throwaway mux so they are never exposed on the embedder's mux.
func buildDefaultFlowProvider(
	cacheManager cache.CacheManagerInterface,
	flowFactory flowcore.FlowFactoryInterface,
	graphCache flowcore.GraphCacheInterface,
	reg executor.ExecutorRegistryInterface,
	interceptorReg interceptor.InterceptorRegistryInterface,
) (FlowProvider, error) {
	svc, _, err := flowmgt.Initialize(
		http.NewServeMux(), nil, cacheManager, flowFactory, reg, interceptorReg, graphCache)
	if err != nil {
		return nil, fmt.Errorf("thunderidengine: failed to build declarative flow provider: %w", err)
	}
	return svc, nil
}

// buildDefaultI18nService constructs a read-only, file-backed translation service from the
// configured declarative i18n resources, for use when WithI18nService was not supplied and
// declarative mode is enabled. Its management routes are mounted on a throwaway mux.
func buildDefaultI18nService() (I18nService, error) {
	runtime := config.GetServerRuntime()
	if runtime == nil {
		return nil, fmt.Errorf("thunderidengine: server runtime configuration is not initialized")
	}
	svc, _, err := i18nmgt.Initialize(http.NewServeMux(), runtime.Config.Translation)
	if err != nil {
		return nil, fmt.Errorf("thunderidengine: failed to build declarative i18n service: %w", err)
	}
	return svc, nil
}

func buildInterceptorRegistry(
	flowFactory flowcore.FlowFactoryInterface,
) (interceptor.InterceptorRegistryInterface, error) {
	runtime := config.GetServerRuntime()
	if runtime == nil {
		return nil, fmt.Errorf("thunderidengine: server runtime configuration is not initialized")
	}
	return interceptor.Initialize(
		interceptor.InterceptorDependencies{FlowFactory: flowFactory},
		runtime.Config.Flow,
	)
}
