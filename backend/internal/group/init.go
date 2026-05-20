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

package group

import (
	"net/http"
	"strings"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
)

// Initialize initializes the group service and registers its routes.
func Initialize(
	mux *http.ServeMux,
	dbProvider provider.DBProviderInterface,
	ouService oupkg.OrganizationUnitServiceInterface,
	entityService entity.EntityServiceInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
	authzService sysauthz.SystemAuthorizationServiceInterface,
) (GroupServiceInterface, oupkg.OUGroupResolver, declarativeresource.ResourceExporter, error) {
	transactioner, err := dbProvider.GetUserDBTransactioner()
	if err != nil {
		return nil, nil, nil, err
	}

	groupStore := newGroupStore()
	groupService := newGroupServiceWithStore(
		groupStore, ouService, entityService, entityTypeService, authzService, transactioner,
	)

	// Create resolver for OU package to query group data without cross-DB access
	ouGroupResolver := newOUGroupResolver(groupStore)

	exporter := newGroupExporter(groupService)

	groupHandler := newGroupHandler(groupService)
	registerRoutes(mux, groupHandler)
	return groupService, ouGroupResolver, exporter, nil
}

// registerRoutes registers the routes for group management operations.
func registerRoutes(mux *http.ServeMux, groupHandler *groupHandler) {
	opts1 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST /groups", groupHandler.HandleGroupPostRequest, opts1))
	mux.HandleFunc(middleware.WithCORS("GET /groups", groupHandler.HandleGroupListRequest, opts1))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /groups", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}, opts1))

	opts2 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	// Special handling for /groups/{id} and /groups/{id}/members
	mux.HandleFunc(middleware.WithCORS("GET /groups/",
		func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, "/groups/")
			segments := strings.Split(path, "/")
			r.SetPathValue("id", segments[0])

			if len(segments) == 1 {
				groupHandler.HandleGroupGetRequest(w, r)
			} else if len(segments) == 2 && segments[1] == "members" {
				groupHandler.HandleGroupMembersGetRequest(w, r)
			} else {
				http.NotFound(w, r)
			}
		}, opts2))
	mux.HandleFunc(middleware.WithCORS("PUT /groups/{id}", groupHandler.HandleGroupPutRequest, opts2))
	mux.HandleFunc(middleware.WithCORS("DELETE /groups/{id}", groupHandler.HandleGroupDeleteRequest, opts2))
	// Handle OPTIONS preflight for /groups/{id} and /groups/{id}/members using the same
	// catch-all pattern as the GET handler above, to avoid conflicts with /groups/tree/{path...}.
	mux.HandleFunc(middleware.WithCORS("OPTIONS /groups/",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts2))

	opts3 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /groups/tree/{path...}",
		groupHandler.HandleGroupListByPathRequest, opts3))
	mux.HandleFunc(middleware.WithCORS("POST /groups/tree/{path...}",
		groupHandler.HandleGroupPostByPathRequest, opts3))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /groups/tree/{path...}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}, opts3))

	// POST routes for /groups/{id}/members/add and /groups/{id}/members/remove.
	// These use a catch-all pattern to avoid route conflicts with /groups/tree/{path...}.
	opts4 := middleware.CORSOptions{
		AllowedMethods:   []string{"POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST /groups/",
		func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(r.URL.Path, "/groups/")
			segments := strings.Split(path, "/")

			// Match /groups/{id}/members/add and /groups/{id}/members/remove
			if len(segments) == 3 && segments[0] != "" && segments[1] == "members" {
				r.SetPathValue("id", segments[0])
				switch segments[2] {
				case "add":
					groupHandler.HandleGroupMembersAddRequest(w, r)
				case "remove":
					groupHandler.HandleGroupMembersRemoveRequest(w, r)
				default:
					http.NotFound(w, r)
				}
			} else {
				http.NotFound(w, r)
			}
		}, opts4))
}
