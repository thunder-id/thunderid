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
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/managedresource"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/system/projectscope"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
)

// Initialize wires up the notification template service, renderer, and exporter and registers routes.
// Only the DB store is wrapped with a read cache for the runtime hot path; the file store is already
// in memory. projectValidator is nil on a deployment that serves no projects, where a template needs no
// project and a project ID is kept unchecked.
//
// bus is required: the service publishes the audit event of every template it creates, updates or
// deletes on it.
func Initialize(mux *http.ServeMux, cacheManager cache.CacheManagerInterface,
	ouService ou.OrganizationUnitServiceInterface,
	authzService sysauthz.SystemAuthorizationServiceInterface, translation translationResolver,
	projectValidator projectOUValidator, bus eventbus.EventBusInterface) (NotificationTemplateServiceInterface,
	TemplateRendererInterface, declarativeresource.ResourceExporter, error) {
	if bus == nil {
		return nil, nil, nil, errNilEventBus
	}
	transactioner, err := provider.GetDBProvider().GetConfigDBTransactioner()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get config database transactioner: %w", err)
	}

	byHandle := cache.GetCache[templateDAO](cacheManager, "NotificationTemplateByHandleCache")
	requireProject := projectValidator != nil &&
		projectscope.ProjectRequired(security.ResourceTypeNotificationTemplate)
	store, err := selectStore(byHandle, requireProject, ouService)
	if err != nil {
		return nil, nil, nil, err
	}

	service := newNotificationTemplateService(store, transactioner, ouService, authzService, projectValidator,
		requireProject, bus)
	service.(*notificationTemplateService).managed = managedresource.GetStore()
	handler := newNotificationTemplateHandler(service)
	registerRoutes(mux, handler)

	templateRenderer := newTemplateRenderer(store, translation)
	exporter := newNotificationTemplateExporter(service)

	return service, templateRenderer, exporter, nil
}

// selectStore builds the store for the configured mode, loading declared templates at startup. The
// DB store is wrapped with the read cache (the file store is already in memory, so it is not cached).
// requireProject refuses a declared template without a project, and systemOUs places a declared template in a
// project without an ouId.
func selectStore(byHandle cache.CacheInterface[templateDAO], requireProject bool, systemOUs systemOUProvider) (
	notificationTemplateStoreInterface, error) {
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
	if err := loadDeclarativeTemplates(fileStore, dbStore, requireProject, systemOUs); err != nil {
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
