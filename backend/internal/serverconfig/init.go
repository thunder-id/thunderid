// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package serverconfig

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/cache"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize wires the server config store, service, and routes, with one handler per supported
// section injected at construction. In declarative and composite modes it loads the read-only
// declarative resources into the file store before serving. It returns the service and the resource
// exporter for registration at the composition root.
func Initialize(mux *http.ServeMux, cacheManager cache.CacheManagerInterface,
	handlers map[ConfigName]ServerConfigHandlerInterface) (
	ServerConfigService, declarativeresource.ResourceExporter, error) {
	store, err := initializeStore(cacheManager, handlers)
	if err != nil {
		return nil, nil, err
	}
	service := newServerConfigService(store, handlers)

	handler := newServerConfigHandler(service)
	registerRoutes(mux, handler)

	return service, newServerConfigExporter(service), nil
}

// initializeStore selects the backing store based on the configured store mode: a mutable (db) store, a
// declarative (file) store, or a composite of both. The writable layer is always present in the mutable
// and composite modes selected by getServerConfigStoreMode. Declarative resources are loaded into the
// file store in the declarative and composite modes.
func initializeStore(cacheManager cache.CacheManagerInterface,
	handlers map[ConfigName]ServerConfigHandlerInterface) (serverConfigStoreInterface, error) {
	cachedDB := func() serverConfigStoreInterface {
		configCache := cache.GetCache[*ServerConfig](cacheManager, serverConfigCacheName)
		return newCachedBackStore(newServerConfigStore(), configCache)
	}

	mode, err := getServerConfigStoreMode()
	if err != nil {
		return nil, err
	}
	switch mode {
	case serverconst.StoreModeDeclarative:
		fileStore := newFileBasedStore()
		if err := seedCORSFromDeploymentConfig(fileStore, handlers); err != nil {
			return nil, err
		}
		if err := loadDeclarativeResources(fileStore, handlers); err != nil {
			return nil, err
		}
		return fileStore, nil
	case serverconst.StoreModeComposite:
		fileStore := newFileBasedStore()
		if err := seedCORSFromDeploymentConfig(fileStore, handlers); err != nil {
			return nil, err
		}
		if err := loadDeclarativeResources(fileStore, handlers); err != nil {
			return nil, err
		}
		return newCompositeServerConfigStore(fileStore, cachedDB()), nil
	default:
		db := cachedDB()
		if err := seedMutableCORSFromDeploymentConfig(db, handlers); err != nil {
			return nil, err
		}
		return db, nil
	}
}

// registerRoutes registers the routes for server config operations.
func registerRoutes(mux *http.ServeMux, handler *serverConfigHandler) {
	noContent := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}

	listOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /server-config", handler.HandleListServerConfigs, listOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /server-config", noContent, listOpts))

	itemOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /server-config/{name}", handler.HandleGetServerConfig, itemOpts))
	mux.HandleFunc(middleware.WithCORS("PUT /server-config/{name}", handler.HandleUpdateServerConfig, itemOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /server-config/{name}", noContent, itemOpts))
}
