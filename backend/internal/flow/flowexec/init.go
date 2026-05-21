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

package flowexec

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/executor"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	"github.com/thunder-id/thunderid/internal/system/config"
	dbprovider "github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/kmprovider"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/system/observability"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

// InitOptions configures optional flow execution initialization overrides.
type InitOptions struct {
	ContextStore ContextStoreInterface
}

// Initialize creates and configures the flow execution service components.
// The observabilitySvc parameter is optional (can be nil) - if nil, observability events won't be published.
func Initialize(
	mux *http.ServeMux,
	flowMgtService flowmgt.FlowMgtServiceInterface,
	inboundClientService inboundclient.InboundClientServiceInterface,
	entityProvider entityprovider.EntityProviderInterface,
	executorRegistry executor.ExecutorRegistryInterface,
	observabilitySvc observability.ObservabilityServiceInterface,
	cryptoSvc kmprovider.RuntimeCryptoProvider,
	opts *InitOptions,
) (FlowExecServiceInterface, error) {
	flowStore, transactioner, err := resolveFlowContextStore(opts)
	if err != nil {
		return nil, err
	}
	flowEngine := newFlowEngine(executorRegistry, observabilitySvc)
	flowExecService := newFlowExecService(flowMgtService, flowStore, flowEngine,
		inboundClientService, entityProvider, observabilitySvc, transactioner, cryptoSvc)

	handler := newFlowExecutionHandler(flowExecService)
	registerRoutes(mux, handler)

	return flowExecService, nil
}

// NewDefaultContextStoreFromConfig returns the flow context store for the configured runtime database.
func NewDefaultContextStoreFromConfig() (ContextStoreInterface, error) {
	if config.GetServerRuntime().Config.Database.Runtime.Type == dbprovider.DataSourceTypeRedis {
		return newRedisFlowStore(dbprovider.GetRedisProvider()), nil
	}
	return newFlowStore(dbprovider.GetDBProvider()), nil
}

func resolveFlowContextStore(opts *InitOptions) (ContextStoreInterface, transaction.Transactioner, error) {
	if opts != nil && opts.ContextStore != nil {
		if config.GetServerRuntime().Config.Database.Runtime.Type == dbprovider.DataSourceTypeRedis {
			return opts.ContextStore, transaction.NewNoOpTransactioner(), nil
		}
		dbProvider := dbprovider.GetDBProvider()
		transactioner, err := dbProvider.GetRuntimeDBTransactioner()
		if err != nil {
			return nil, nil, err
		}
		return opts.ContextStore, transactioner, nil
	}

	if config.GetServerRuntime().Config.Database.Runtime.Type == dbprovider.DataSourceTypeRedis {
		return newRedisFlowStore(dbprovider.GetRedisProvider()), transaction.NewNoOpTransactioner(), nil
	}
	dbProvider := dbprovider.GetDBProvider()
	transactioner, err := dbProvider.GetRuntimeDBTransactioner()
	if err != nil {
		return nil, nil, err
	}
	return newFlowStore(dbProvider), transactioner, nil
}

func registerRoutes(mux *http.ServeMux, handler *flowExecutionHandler) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST /flow/execute",
		middleware.CorrelationIDMiddleware(http.HandlerFunc(handler.HandleFlowExecutionRequest)).ServeHTTP, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /flow/execute",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
}
