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

// Package envmgr promotes configuration between deployments: it captures a control plane's
// configuration as a version, diffs versions, and applies one to a data plane. It is the standalone
// environment manager service brought in-process; the standalone one still exists and is unchanged.
package envmgr

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/envmgr/auth"
	"github.com/thunder-id/thunderid/internal/envmgr/service"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize mounts the environment manager on the given mux and returns its service.
//
// The hasher is what lets a credential that is only ever verified, such as an application's client
// secret, be set from here: it is hashed the same way the server hashes one it captures itself.
func Initialize(mux *http.ServeMux, hasher service.SecretHasher) (*registry, error) {
	reg := newRegistry(hasher)
	registerRoutes(mux, reg)
	return reg, nil
}

// registerRoutes mounts each route on the server's own mux rather than delegating a subtree, so
// these paths sit alongside the rest of the management API instead of behind a prefix.
//
// Every route records the caller's bearer token: the capture and apply paths call a control plane
// and a data plane as the caller, and without the token those calls arrive unauthenticated.
func registerRoutes(mux *http.ServeMux, reg *registry) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	routes := map[string]func(*Server) http.HandlerFunc{
		"POST /environments":                    func(s *Server) http.HandlerFunc { return s.createEnvironment },
		"GET /environments":                     func(s *Server) http.HandlerFunc { return s.listEnvironments },
		"GET /environments/{id}":                func(s *Server) http.HandlerFunc { return s.getEnvironment },
		"GET /data-plane-jobs/{id}":             func(s *Server) http.HandlerFunc { return s.getDataPlaneJob },
		"PUT /environments/{id}/edges":          func(s *Server) http.HandlerFunc { return s.updateEnvironmentEdges },
		"DELETE /environments/{id}":             func(s *Server) http.HandlerFunc { return s.deleteEnvironment },
		"POST /environments/{id}/versions":      func(s *Server) http.HandlerFunc { return s.createVersion },
		"GET /environments/{id}/versions":       func(s *Server) http.HandlerFunc { return s.listVersions },
		"GET /environments/{id}/versions/{seq}": func(s *Server) http.HandlerFunc { return s.getVersion },
		"GET /environments/{id}/diff":           func(s *Server) http.HandlerFunc { return s.diff },
		"GET /environments/{id}/variables":      func(s *Server) http.HandlerFunc { return s.checkVariables },
		"POST /environments/{id}/apply":         func(s *Server) http.HandlerFunc { return s.apply },
		"GET /environments/{id}/secrets":        func(s *Server) http.HandlerFunc { return s.listSecrets },
		"POST /environments/{id}/data-plane-token": func(s *Server) http.HandlerFunc {
			return s.regenerateDataPlaneToken
		},
		"PUT /environments/{id}/secrets/{name}": func(s *Server) http.HandlerFunc { return s.setSecret },
		"POST /environments/{id}/secrets/{name}/regenerate": func(s *Server) http.HandlerFunc {
			return s.regenerateSecret
		},
		"POST /environments/{id}/revert": func(s *Server) http.HandlerFunc { return s.revert },
		"POST /environments/{id}/managed": func(s *Server) http.HandlerFunc {
			return s.setManagedEnvironment
		},
		"POST /apply":                                func(s *Server) http.HandlerFunc { return s.applyAll },
		"GET /promotions/preview":                    func(s *Server) http.HandlerFunc { return s.promotePreview },
		"POST /promotions":                           func(s *Server) http.HandlerFunc { return s.promote },
		"PUT /tenants/{deploymentId}/secrets/{name}": func(s *Server) http.HandlerFunc { return s.captureSecret },
	}
	// Promotion is the one action gated on a scope; see requirePromotionScope.
	promotionRoutes := map[string]bool{
		"GET /promotions/preview": true,
		"POST /promotions":        true,
	}
	for pattern, pick := range routes {
		handler := reg.handler(pick)
		if promotionRoutes[pattern] {
			handler = requirePromotionScope(handler)
		}
		mux.HandleFunc(middleware.WithCORS(pattern, auth.RecordCallerToken(handler), opts))
	}

	for _, pattern := range []string{"OPTIONS /environments", "OPTIONS /environments/",
		"OPTIONS /promotions", "OPTIONS /apply"} {
		mux.HandleFunc(middleware.WithCORS(pattern, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
	}
}
