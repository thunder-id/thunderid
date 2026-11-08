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
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/sharing"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	dbprovider "github.com/thunder-id/thunderid/internal/system/database/provider"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/system/observability"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
)

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
) (AgentServiceInterface, declarativeresource.ResourceExporter, error) {
	if bus == nil {
		return nil, nil, errNilEventBus
	}
	transactioner, err := dbProvider.GetEntityDBTransactioner()
	if err != nil {
		return nil, nil, err
	}
	service := newAgentService(authzService, entityService, inboundClientService, ouService, roleService,
		governance, valueCapturer, entityTypeService, transactioner, observabilitySvc, projectValidator, bus)

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
}
