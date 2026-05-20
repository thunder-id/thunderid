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

package agent

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize wires the agent service, registers HTTP routes and returns the service.
func Initialize(
	mux *http.ServeMux,
	entityService entity.EntityServiceInterface,
	inboundClientService inboundclient.InboundClientServiceInterface,
	ouService oupkg.OrganizationUnitServiceInterface,
) (AgentServiceInterface, error) {
	service := newAgentService(entityService, inboundClientService, ouService)
	handler := newAgentHandler(service)
	registerRoutes(mux, handler)
	return service, nil
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
}
