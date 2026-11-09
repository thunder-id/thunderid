// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/thunder-id/thunderid/internal/serverconfig"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/export"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize wires the gateway store, service and routes.
//
// Which store depends on gateway.store:
//
//  1. mutable: the database alone. Gateways declared in files are not read.
//  2. declarative: files alone. Registration through the API is refused, because there is nowhere
//     to put it that a restart would not discard.
//  3. composite: both. Reads merge the two, registration writes to the database, and a gateway a
//     file declared cannot be changed or removed through the API.
//
// The exporter is what a version is captured through, so a version holds exactly what this plane's
// export writes: references on a control plane, template placeholders with their values elsewhere.
//
// capture is bound here when given. Only a control plane gives one: its export refers to values the
// default gateway has to hold, while a plane exporting template placeholders carries the values
// itself and has nothing to put anywhere.
// The exporters it was built from are what show a gateway's applied resources as their own reads do.
//
// projects, when given, are what a deploy can be scoped to. Only a control plane gives them.
//
// changes is bound here when given, so the configuration is captured as the current version after
// each change. Only a control plane gives one: it is where the configuration is authored.
//
// settings is the server-config service the gateway section is read from, so auto apply can be turned
// on and off at runtime; without it, gateway.auto_apply alone decides.
//
// The same operations are registered on mcpServer, when given, as tools that answer as the REST endpoints do, so
// an MCP client can capture, deploy, promote and roll back as the console does.
//
// bus is required: the version service publishes the audit event of every apply and revert it is asked
// for on it.
func Initialize(mux *http.ServeMux, mcpServer *mcp.Server, exporter export.ExportServiceInterface,
	capture *ValueCapture, exporters []declarativeresource.ResourceExporter,
	changes *ChangeCapture, settings serverconfig.ServerConfigService,
	projects ProjectDirectory, bus eventbus.EventBusInterface) (ServiceInterface, error) {
	if bus == nil {
		return nil, errNilEventBus
	}
	var (
		gatewayStore storeInterface
		fileStore    *gatewayFileStore
	)

	switch getGatewayStoreMode() {
	case serverconst.StoreModeDeclarative:
		fileStore = newFileStore()
		gatewayStore = fileStore
	case serverconst.StoreModeComposite:
		fileStore = newFileStore()
		gatewayStore = newCompositeStore(fileStore, newStore())
	default:
		gatewayStore = newStore()
	}

	// One pipeline, so the one the API shows is the one auto apply and value capture deploy to.
	deployPipeline := newPipeline()
	service := newService(gatewayStore, deployPipeline)
	versions := newVersionService(gatewayStore, newVersionStore(), exporter, newGatewayClient(),
		declarativeresource.SecretFieldsOf(exporters), newActorNames(), deployPipeline, bus)
	versions.autoApply = newAutoApplySource(settings)
	versions.syncSettings = settings
	if projects != nil {
		versions.projects = projects
	}
	h := newHandler(service)
	h.afterDelete = versions.Forget
	stores := newStoreService(gatewayStore, newGatewayClient())
	if capture != nil {
		capture.bind(exporter, gatewayStore, stores, deployPipeline)
	}
	if changes != nil {
		changes.bind(versions)
	}
	registerRoutes(mux, h)
	versionRoutes := newVersionHandler(versions)
	registerVersionRoutes(mux, versionRoutes)
	registerMasterRoutes(mux, versionRoutes)
	applied := newAppliedConfigurationService(versions, exporters)
	registerAppliedConfigurationRoutes(mux, newAppliedConfigurationHandler(applied))
	registerStoreRoutes(mux, newStoreHandler(stores))
	if mcpServer != nil {
		registerMCPTools(mcpServer, &gatewayTools{gateways: service, versions: versions, applied: applied,
			stores: stores})
	}

	// A gateway can also be declared in a file rather than registered through the API. The files are
	// read on every start into the in-memory store, so the file is the whole truth about what it
	// declares and re-reading replaces rather than accumulates.
	if fileStore != nil {
		if err := loadDeclarativeGateways(fileStore); err != nil {
			return service, err
		}
	}
	// After the declared gateways are loaded, so the first gateway can be one of them.
	if capture != nil {
		go capture.fillMissingVariables(context.Background(), exporters)
	}
	return service, nil
}

func registerRoutes(mux *http.ServeMux, h *handler) {
	noContent := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}

	collectionOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /gateways", h.handleList, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("POST /gateways", h.handleRegister, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /gateways", noContent, collectionOpts))

	pipelineOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /gateways/pipeline", h.handlePipeline, pipelineOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /gateways/pipeline", noContent, pipelineOpts))

	itemOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /gateways/{id}", h.handleGet, itemOpts))
	mux.HandleFunc(middleware.WithCORS("PUT /gateways/{id}", h.handleUpdate, itemOpts))
	mux.HandleFunc(middleware.WithCORS("DELETE /gateways/{id}", h.handleDelete, itemOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /gateways/{id}", noContent, itemOpts))
}

func registerVersionRoutes(mux *http.ServeMux, h *versionHandler) {
	noContent := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
	readOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	writeOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	collectionOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	mux.HandleFunc(middleware.WithCORS("GET /configuration-versions", h.handleListVersions, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("POST /configuration-versions", h.handleCapture, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /configuration-versions", noContent, collectionOpts))
	mux.HandleFunc(middleware.WithCORS("GET /configuration-versions/{version}", h.handleGetVersion, readOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /configuration-versions/{version}", noContent, readOpts))

	mux.HandleFunc(middleware.WithCORS("GET /gateways/{id}/applied-version", h.handleGetApplied, readOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /gateways/{id}/applied-version", noContent, readOpts))
	mux.HandleFunc(middleware.WithCORS("GET /gateways/{id}/diff", h.handleDiff, readOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /gateways/{id}/diff", noContent, readOpts))
	mux.HandleFunc(middleware.WithCORS("GET /gateways/{id}/project-status", h.handleProjectStatus, readOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /gateways/{id}/project-status", noContent, readOpts))
	mux.HandleFunc(middleware.WithCORS("POST /gateways/{id}/apply", h.handleApply, writeOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /gateways/{id}/apply", noContent, writeOpts))
	mux.HandleFunc(middleware.WithCORS("POST /gateways/{id}/revert", h.handleRevert, writeOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /gateways/{id}/revert", noContent, writeOpts))
	syncOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("PUT /projects/{projectId}/auto-sync", h.handleSetProjectAutoSync,
		syncOpts))
	mux.HandleFunc(middleware.WithCORS("DELETE /projects/{projectId}/auto-sync",
		h.handleDeleteProjectAutoSync, syncOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /projects/{projectId}/auto-sync", noContent, syncOpts))
}

// registerAppliedConfigurationRoutes serves the configuration a gateway runs.
func registerAppliedConfigurationRoutes(mux *http.ServeMux, h *appliedConfigurationHandler) {
	noContent := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
	readOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /gateways/{id}/applied-configuration",
		h.handleGetAppliedConfiguration, readOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /gateways/{id}/applied-configuration", noContent, readOpts))
}

// registerStoreRoutes serves a gateway's variables and secrets, managed through this plane with the
// gateway's key.
func registerStoreRoutes(mux *http.ServeMux, h *storeHandler) {
	noContent := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}
	collectionOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	itemOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	routes := []struct {
		collection                     string
		list, create, get, set, remove http.HandlerFunc
	}{
		{collectionVariables, listHandler(h.service.ListVariables), createHandler(h.service.CreateVariable),
			getHandler(h.service.GetVariable), setHandler(h.service.SetVariable),
			deleteHandler(h.service.DeleteVariable)},
		{collectionSecrets, listHandler(h.service.ListSecrets), createHandler(h.service.CreateSecret),
			getHandler(h.service.GetSecret), setHandler(h.service.SetSecret),
			deleteHandler(h.service.DeleteSecret)},
	}
	for _, route := range routes {
		base := "/gateways/{id}/" + route.collection
		mux.HandleFunc(middleware.WithCORS("GET "+base, route.list, collectionOpts))
		mux.HandleFunc(middleware.WithCORS("POST "+base, route.create, collectionOpts))
		mux.HandleFunc(middleware.WithCORS("OPTIONS "+base, noContent, collectionOpts))
		item := base + "/{name}"
		mux.HandleFunc(middleware.WithCORS("GET "+item, route.get, itemOpts))
		mux.HandleFunc(middleware.WithCORS("PUT "+item, route.set, itemOpts))
		mux.HandleFunc(middleware.WithCORS("DELETE "+item, route.remove, itemOpts))
		mux.HandleFunc(middleware.WithCORS("OPTIONS "+item, noContent, itemOpts))
	}
}
