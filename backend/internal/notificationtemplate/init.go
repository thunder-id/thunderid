// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package notificationtemplate

import (
	"fmt"
	"net/http"

	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/cache"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize wires up the notification template service, renderer, and exporter and registers routes.
// Only the DB store is wrapped with a read cache for the runtime hot path; the file store is already
// in memory.
func Initialize(mux *http.ServeMux, cacheManager cache.CacheManagerInterface,
	ouService ou.OrganizationUnitServiceInterface,
	translation translationResolver) (NotificationTemplateServiceInterface, TemplateRendererInterface,
	declarativeresource.ResourceExporter, error) {
	transactioner, err := provider.GetDBProvider().GetConfigDBTransactioner()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get config database transactioner: %w", err)
	}

	byHandle := cache.GetCache[templateDAO](cacheManager, "NotificationTemplateByHandleCache")
	store, err := selectStore(byHandle)
	if err != nil {
		return nil, nil, nil, err
	}

	service := newNotificationTemplateService(store, transactioner, ouService)
	handler := newNotificationTemplateHandler(service)
	registerRoutes(mux, handler)

	templateRenderer := newTemplateRenderer(store, translation)
	exporter := newNotificationTemplateExporter(service)

	return service, templateRenderer, exporter, nil
}

// selectStore builds the store for the configured mode, loading declared templates at startup. The
// DB store is wrapped with the read cache (the file store is already in memory, so it is not cached).
func selectStore(byHandle cache.CacheInterface[templateDAO]) (notificationTemplateStoreInterface, error) {
	mode, err := getStoreMode()
	if err != nil {
		return nil, err
	}
	if mode != serverconst.StoreModeDeclarative && mode != serverconst.StoreModeComposite {
		return newCacheBackedStore(byHandle, newNotificationTemplateStore()), nil
	}

	fileStore := newFileBasedTemplateStore()
	var dbStore notificationTemplateStoreInterface
	if mode == serverconst.StoreModeComposite {
		dbStore = newCacheBackedStore(byHandle, newNotificationTemplateStore())
	}

	// Load declared templates once; in composite mode dbStore is non-nil so a declared handle already
	// taken by a database template fails the load.
	if err := loadDeclarativeTemplates(fileStore, dbStore); err != nil {
		return nil, err
	}

	if mode == serverconst.StoreModeComposite {
		return newCompositeStore(fileStore, dbStore), nil
	}
	return fileStore, nil
}

// registerRoutes registers the notification template management routes.
func registerRoutes(mux *http.ServeMux, handler *notificationTemplateHandler) {
	if mux == nil {
		return
	}

	collectionOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /notification-templates/{channel}/templates",
		handler.HandleTemplateListRequest, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("POST /notification-templates/{channel}/templates",
		handler.HandleTemplatePostRequest, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /notification-templates/{channel}/templates",
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }, collectionOpts))

	itemOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /notification-templates/{channel}/templates/{id}",
		handler.HandleTemplateGetRequest, itemOpts))
	mux.HandleFunc(middleware.WithCORS("PUT /notification-templates/{channel}/templates/{id}",
		handler.HandleTemplatePutRequest, itemOpts))
	mux.HandleFunc(middleware.WithCORS("DELETE /notification-templates/{channel}/templates/{id}",
		handler.HandleTemplateDeleteRequest, itemOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /notification-templates/{channel}/templates/{id}",
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }, itemOpts))
}
