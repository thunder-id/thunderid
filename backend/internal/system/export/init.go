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

// Package export provides functionality for exporting various resource configurations.
package export

import (
	"net/http"

	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize initializes the export service and registers its routes.
func Initialize(mux *http.ServeMux, exporters []declarativeresource.ResourceExporter) ExportServiceInterface {
	// Create parameterizer instance (no longer needs centralized rules)
	parameterizerInstance := newParameterizer(templatingRules{})

	// Create the export service with exporters
	exportService := newExportService(exporters, parameterizerInstance)

	// Create the handler
	exportHandler := newExportHandler(exportService)

	// Register routes
	registerRoutes(mux, exportHandler)

	return exportService
}

func registerRoutes(mux *http.ServeMux, exportHandler *exportHandler) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	// JSON export endpoint
	mux.HandleFunc(middleware.WithCORS("POST /export",
		exportHandler.HandleExportRequest, opts))

	// ZIP export endpoint - returns application/zip with individual files
	mux.HandleFunc(middleware.WithCORS("POST /export/zip",
		exportHandler.HandleExportZipRequest, opts))

	mux.HandleFunc(middleware.WithCORS("OPTIONS /export",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /export/zip",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
}
