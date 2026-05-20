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

package flowmgt

import (
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/executor"

	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

// Initialize initializes the flow management service and registers HTTP routes.
func Initialize(
	mux *http.ServeMux,
	mcpServer *mcp.Server,
	cacheManager cache.CacheManagerInterface,
	flowFactory core.FlowFactoryInterface,
	executorRegistry executor.ExecutorRegistryInterface,
	graphCache core.GraphCacheInterface,
) (FlowMgtServiceInterface, declarativeresource.ResourceExporter, error) {
	store, compositeStore, transactioner, err := initializeStore(cacheManager)
	if err != nil {
		return nil, nil, err
	}

	inferenceService := newFlowInferenceService()
	graphBuilder := newGraphBuilder(flowFactory, executorRegistry, graphCache)
	service := newFlowMgtService(store, inferenceService, graphBuilder, executorRegistry, compositeStore, transactioner)

	handler := newFlowMgtHandler(service)
	registerRoutes(mux, handler)

	// Register MCP tools
	if mcpServer != nil {
		registerMCPTools(mcpServer, service)
	}

	// Create and return exporter
	exporter := newFlowGraphExporter(service)
	return service, exporter, nil
}

// Store Selection (based on flow.store configuration):
//
// 1. MUTABLE mode (store: "mutable"):
//   - Uses database store only
//   - Supports full CRUD operations
//   - All flows are mutable
//
// 2. IMMUTABLE mode (store: "declarative"):
//   - Uses file-based store only
//   - All flows are immutable (read-only)
//   - No create/update/delete operations allowed
//
// 3. COMPOSITE mode (store: "composite" - hybrid):
//   - Uses both file-based store (immutable) + database store (mutable)
//   - YAML resources are loaded into file-based store
//   - Database store handles runtime flows
//   - Reads check both stores (merged results)
//   - Writes only go to database store
//   - Declarative flows cannot be updated or deleted
func initializeStore(cacheManager cache.CacheManagerInterface) (
	flowStoreInterface, *compositeFlowStore, transaction.Transactioner, error) {
	var compositeStore *compositeFlowStore

	storeMode := getFlowStoreMode()

	flowByIDCache := cache.GetCache[*CompleteFlowDefinition](cacheManager, "FlowByIDCache")
	flowByHandleCache := cache.GetCache[*CompleteFlowDefinition](cacheManager, "FlowByHandleCache")

	switch storeMode {
	case serverconst.StoreModeComposite:
		fileStore, _ := newFileBasedStore()
		dbStore, transactioner, err := newCacheBackedFlowStore(flowByIDCache, flowByHandleCache)
		if err != nil {
			return nil, nil, nil, err
		}
		compositeStore = newCompositeFlowStore(fileStore, dbStore)
		if err := loadDeclarativeResources(fileStore); err != nil {
			return nil, nil, nil, err
		}
		return compositeStore, compositeStore, transactioner, nil

	case serverconst.StoreModeDeclarative:
		fileStore, transactioner := newFileBasedStore()
		if err := loadDeclarativeResources(fileStore); err != nil {
			return nil, nil, nil, err
		}
		return fileStore, nil, transactioner, nil

	default:
		store, transactioner, err := newCacheBackedFlowStore(flowByIDCache, flowByHandleCache)
		if err != nil {
			return nil, nil, nil, err
		}
		return store, nil, transactioner, nil
	}
}

// getFlowStoreMode determines the store mode for flows.
// Resolution order:
//  1. If Flow.Store is explicitly configured, use it
//  2. Otherwise, fall back to global DeclarativeResources.Enabled
func getFlowStoreMode() serverconst.StoreMode {
	cfg := config.GetServerRuntime().Config
	// Check if service-level configuration is explicitly set
	if cfg.Flow.Store != "" {
		mode := serverconst.StoreMode(strings.ToLower(strings.TrimSpace(cfg.Flow.Store)))
		// Validate and normalize
		switch mode {
		case serverconst.StoreModeMutable, serverconst.StoreModeDeclarative, serverconst.StoreModeComposite:
			return mode
		}
	}

	// Fall back to global declarative resources setting
	if declarativeresource.IsDeclarativeModeEnabled() {
		return serverconst.StoreModeDeclarative
	}

	return serverconst.StoreModeMutable
}

// isCompositeModeEnabled checks if composite store mode is enabled for flows.
func isCompositeModeEnabled() bool {
	return getFlowStoreMode() == serverconst.StoreModeComposite
}

// registerRoutes registers the HTTP routes for flow management.
func registerRoutes(mux *http.ServeMux, handler *flowMgtHandler) {
	opts1 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /flows", handler.listFlows, opts1))
	mux.HandleFunc(middleware.WithCORS("POST /flows", handler.createFlow, opts1))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /flows", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}, opts1))

	opts2 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /flows/{flowId}", handler.getFlow, opts2))
	mux.HandleFunc(middleware.WithCORS("PUT /flows/{flowId}", handler.updateFlow, opts2))
	mux.HandleFunc(middleware.WithCORS("DELETE /flows/{flowId}", handler.deleteFlow, opts2))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /flows/{flowId}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}, opts2))

	opts3 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /flows/{flowId}/versions", handler.listFlowVersions, opts3))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /flows/{flowId}/versions",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts3),
	)
	mux.HandleFunc(middleware.WithCORS("GET /flows/{flowId}/versions/{version}", handler.getFlowVersion, opts3))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /flows/{flowId}/versions/{version}",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts3),
	)

	opts4 := middleware.CORSOptions{
		AllowedMethods:   []string{"POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST /flows/{flowId}/restore", handler.restoreFlowVersion, opts4))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /flows/{flowId}/restore",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts4),
	)
}
