// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	"github.com/thunder-id/thunderid/internal/identitygovernance"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/sharing"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	dbprovider "github.com/thunder-id/thunderid/internal/system/database/provider"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/managedresource"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/system/observability"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
)

// Option adjusts the agent service for the plane it runs on.
type Option func(*agentService)

// KeepingValueReferences is for a control plane, which keeps a reference to the owner each deployment
// holds where the owner's values belong.
func KeepingValueReferences() Option {
	return func(s *agentService) { s.keepsReferences = true }
}

// Initialize wires the agent service, registers HTTP routes and returns the service and exporter.
//
// mcpServer is optional. Given one, the agent MCP tools are registered with it, and they name APIs
// through resourceService.
//
// valueCapturer is optional. Given one, the service hands it every agent it creates or changes, so the
// values the agent's export refers to are kept where a reference finds them.
//
// projectValidator is nil on a deployment that serves no projects, where a project ID is kept unchecked.
//
// bus is required: the service publishes the audit event of every agent it creates, updates or deletes
// on it.
//
// revoker is optional. Without one, which is the control plane, owners cannot suspend their agents and those
// routes are not registered.
//
// artifactLifetime is optional. Given one, a revocation of an agent's tokens is kept as long as they can live.
func Initialize(
	mux *http.ServeMux,
	mcpServer *mcp.Server,
	entityService entity.EntityServiceInterface,
	sharingService sharing.SharingServiceInterface,
	inboundClientService inboundclient.InboundClientServiceInterface,
	ouService oupkg.OrganizationUnitServiceInterface,
	roleService role.RoleServiceInterface,
	resourceService resource.ResourceServiceInterface,
	flowService flowmgt.FlowMgtServiceInterface,
	authzService sysauthz.SystemAuthorizationServiceInterface,
	governance identitygovernance.IdentityGovernanceServiceInterface,
	valueCapturer declarativeresource.ValueCapturer,
	entityTypeService entitytype.EntityTypeServiceInterface,
	dbProvider dbprovider.DBProviderInterface,
	observabilitySvc observability.ObservabilityServiceInterface,
	projectValidator projectOUValidator,
	bus eventbus.EventBusInterface,
	revoker revocation.CriteriaRevoker,
	artifactLifetime artifactLifetimeResolver,
	opts ...Option,
) (AgentServiceInterface, declarativeresource.ResourceExporter, error) {
	if bus == nil {
		return nil, nil, errNilEventBus
	}
	transactioner, err := dbProvider.GetEntityDBTransactioner()
	if err != nil {
		return nil, nil, err
	}
	service := newAgentService(authzService, entityService, inboundClientService, ouService, sharingService,
		roleService, governance, valueCapturer, entityTypeService, transactioner, observabilitySvc, projectValidator,
		bus, revoker, artifactLifetime)
	service.(*agentService).managed = managedresource.GetStore()
	for _, opt := range opts {
		opt(service.(*agentService))
	}

	if err := entityService.LoadIndexedAttributes(getAgentIndexedAttributes()); err != nil {
		return nil, nil, err
	}
	sharingService.RegisterResourceType(newAgentSharingDeclaration(service))
	entityService.RegisterSharedCategory(providers.EntityCategoryAgent, agentSharingType)

	storeMode := getAgentStoreMode()
	if storeMode == serverconst.StoreModeComposite || storeMode == serverconst.StoreModeDeclarative {
		if err := entityService.LoadDeclarativeResources(makeAgentDeclarativeConfig(service)); err != nil {
			return nil, nil, err
		}
		if err := inboundClientService.LoadDeclarativeResources(
			context.Background(), makeAgentInboundConfig(service)); err != nil {
			return nil, nil, err
		}
		// Read after the agents, as each policy is declared against the unit its agent was loaded into.
		if err := sharingService.LoadDeclarativeResources(
			context.Background(), makeAgentSharingConfig(service)); err != nil {
			return nil, nil, err
		}
	}

	handler := newAgentHandler(service)
	registerRoutes(mux, handler)
	if mcpServer != nil {
		registerMCPTools(mcpServer, &agentTools{
			logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "AgentMCPTools")),
			agentService:      service,
			entityService:     entityService,
			entityTypeService: entityTypeService,
			roleService:       roleService,
			resourceService:   resourceService,
			flowService:       flowService,
		})
	}
	if revoker != nil {
		registerOwnerSuspensionRoutes(mux, handler)
	}

	exporter := newAgentExporter(service, entityService, sharingService)
	return service, exporter, nil
}

func registerRoutes(mux *http.ServeMux, h *agentHandler) {
	listOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /agents", h.HandleAgentListRequest, listOpts))
	mux.HandleFunc(middleware.WithCORS("POST /agents", h.HandleAgentPostRequest, listOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /agents",
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, listOpts))

	itemOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /agents/{id}", h.HandleAgentGetRequest, itemOpts))
	mux.HandleFunc(middleware.WithCORS("PUT /agents/{id}", h.HandleAgentPutRequest, itemOpts))
	mux.HandleFunc(middleware.WithCORS("DELETE /agents/{id}", h.HandleAgentDeleteRequest, itemOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /agents/",
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, itemOpts))

	secretOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST /agents/{id}/client-secret/regenerate",
		h.HandleClientSecretRegenerateRequest, secretOpts))
	// Without its own preflight route this action falls to OPTIONS /agents/, which does not allow POST,
	// and the browser blocks the request.
	mux.HandleFunc(middleware.WithCORS("OPTIONS /agents/{id}/client-secret/regenerate",
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, secretOpts))

	groupsOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /agents/{id}/groups",
		h.HandleAgentGroupsRequest, groupsOpts))
	mux.HandleFunc(middleware.WithCORS("GET /agents/{id}/roles",
		h.HandleAgentRolesRequest, groupsOpts))

	ownedListOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /users/me/agents", h.HandleOwnedAgentListRequest, ownedListOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /users/me/agents",
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, ownedListOpts))

	ownedItemOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /users/me/agents/{id}", h.HandleOwnedAgentGetRequest, ownedItemOpts))
	mux.HandleFunc(middleware.WithCORS("PUT /users/me/agents/{id}", h.HandleOwnedAgentPutRequest, ownedItemOpts))
	mux.HandleFunc(middleware.WithCORS("DELETE /users/me/agents/{id}",
		h.HandleOwnedAgentDeleteRequest, ownedItemOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /users/me/agents/",
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, ownedItemOpts))
}

// registerOwnerSuspensionRoutes registers the routes an owner suspends and unsuspends their agents with.
func registerOwnerSuspensionRoutes(mux *http.ServeMux, h *agentHandler) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST /users/me/agents/suspend", h.HandleOwnedAgentsSuspendRequest, opts))
	mux.HandleFunc(middleware.WithCORS("POST /users/me/agents/{id}/suspend",
		h.HandleOwnedAgentSuspendRequest, opts))
	mux.HandleFunc(middleware.WithCORS("POST /users/me/agents/{id}/unsuspend",
		h.HandleOwnedAgentUnsuspendRequest, opts))
}
