// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"net/http"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/identitygovernance"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/role"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
)

// Initialize wires the agent service, registers HTTP routes and returns the service and exporter.
//
// valueCapturer is optional. Given one, the service hands it every agent it creates or changes, so the
// values the agent's export refers to are kept where a reference finds them.
//
// revoker is optional. Without one, which is the control plane, owners cannot suspend their agents and those
// routes are not registered.
//
// artifactLifetime is optional. Given one, a revocation of an agent's tokens is kept as long as they can live.
func Initialize(
	mux *http.ServeMux,
	entityService entity.EntityServiceInterface,
	inboundClientService inboundclient.InboundClientServiceInterface,
	ouService oupkg.OrganizationUnitServiceInterface,
	roleService role.RoleServiceInterface,
	authzService sysauthz.SystemAuthorizationServiceInterface,
	governance identitygovernance.IdentityGovernanceServiceInterface,
	valueCapturer declarativeresource.ValueCapturer,
	entityTypeService entitytype.EntityTypeServiceInterface,
	revoker revocation.CriteriaRevoker,
	artifactLifetime artifactLifetimeResolver,
) (AgentServiceInterface, declarativeresource.ResourceExporter, error) {
	service := newAgentService(authzService, entityService, inboundClientService, ouService, roleService,
		governance, valueCapturer, entityTypeService, revoker, artifactLifetime)

	if err := entityService.LoadIndexedAttributes(getAgentIndexedAttributes()); err != nil {
		return nil, nil, err
	}

	storeMode := getAgentStoreMode()
	if storeMode == serverconst.StoreModeComposite || storeMode == serverconst.StoreModeDeclarative {
		if err := entityService.LoadDeclarativeResources(makeAgentDeclarativeConfig(service)); err != nil {
			return nil, nil, err
		}
		if err := inboundClientService.LoadDeclarativeResources(
			context.Background(), makeAgentInboundConfig(service)); err != nil {
			return nil, nil, err
		}
	}

	handler := newAgentHandler(service)
	registerRoutes(mux, handler)
	if revoker != nil {
		registerOwnerSuspensionRoutes(mux, handler)
	}

	exporter := newAgentExporter(service)
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
