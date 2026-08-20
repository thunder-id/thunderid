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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package managedresource

import (
	"encoding/json"
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// listResponse reports which resources this deployment does not own, grouped by resource type.
//
// It is one call rather than a flag on every resource because a console needs the answer for a whole
// page at once, and every resource type would otherwise have to carry the same field.
type listResponse struct {
	// Enabled reports whether ownership is tracked at all. When false the lists are empty because
	// nothing is owned elsewhere, not because nothing was found.
	Enabled bool `json:"enabled"`
	// Managed maps a resource type to the ids owned by the control plane.
	Managed map[string][]string `json:"managed"`
}

// allTypes are the resource types reported. A type with nothing managed reports an empty list, so a
// caller can tell "none" apart from "not a type I know".
var allTypes = []string{
	TypeOrganizationUnit, TypeEntityType, TypeResourceServer, TypeRole, TypeGroup,
	TypeConnection, TypeFlow, TypeTheme, TypeLayout, TypeApplication, TypeUser,
	TypeTranslation, TypeAgent, TypePresentationDefinition, TypeCredentialConfiguration,
	TypeServerConfig,
}

// RegisterRoutes exposes the read-only listing under /managed-resources.
func RegisterRoutes(mux *http.ServeMux) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /managed-resources", handleList, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /managed-resources",
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }, opts))
}

// handleList answers with everything the control plane owns on this deployment.
func handleList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	registry := Default()

	response := listResponse{Enabled: registry.Enabled(), Managed: map[string][]string{}}
	if registry.Enabled() {
		for _, resourceType := range allTypes {
			ids := registry.ManagedIDs(ctx, resourceType)
			list := make([]string, 0, len(ids))
			for id := range ids {
				list = append(list, id)
			}
			response.Managed[resourceType] = list
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.GetLogger().Error(ctx, "Failed to write the managed resource listing", log.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
	}
}
