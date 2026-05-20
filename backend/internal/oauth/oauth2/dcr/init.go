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

// Package dcr provides Dynamic Client Registration (DCR) implementation.
package dcr

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/application"
	"github.com/thunder-id/thunderid/internal/ou"
	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

// Initialize initializes the DCR service and registers its routes.
func Initialize(
	mux *http.ServeMux,
	appService application.ApplicationServiceInterface,
	ouService ou.OrganizationUnitServiceInterface,
	i18nService i18nmgt.I18nServiceInterface,
	transactioner transaction.Transactioner,
) DCRServiceInterface {
	dcrService := newDCRService(appService, ouService, i18nService, transactioner)
	dcrHandler := newDCRHandler(dcrService)
	registerRoutes(mux, dcrHandler)
	return dcrService
}

// registerRoutes registers the routes for DCR operations.
func registerRoutes(mux *http.ServeMux, dcrHandler *dcrHandler) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"POST", "OPTIONS"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST /oauth2/dcr/register",
		dcrHandler.HandleDCRRegistration, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /oauth2/dcr/register",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
}
