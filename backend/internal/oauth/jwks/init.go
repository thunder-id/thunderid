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

package jwks

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/system/kmprovider"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize initializes the JWKS service and registers its routes.
func Initialize(mux *http.ServeMux, cryptoProvider kmprovider.RuntimeCryptoProvider) JWKSServiceInterface {
	// Initialize the JWKS service
	jwksService := newJWKSService(cryptoProvider)
	jwksHandler := newJWKSHandler(jwksService)
	registerRoutes(mux, jwksHandler)
	return jwksService
}

// registerRoutes registers the routes for the JWKSAPIService.
func registerRoutes(mux *http.ServeMux, jwksHandler *jwksHandler) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "OPTIONS"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /oauth2/jwks",
		jwksHandler.HandleJWKSRequest, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /oauth2/jwks",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
}
