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

// Package application provides functionality for managing applications.
package application

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize initializes the application service and registers its routes.
func Initialize(
	mux *http.ServeMux,
	mcpServer *mcp.Server,
	entityProvider entityprovider.EntityProviderInterface,
	entityService entity.EntityServiceInterface,
	inboundClient inboundclient.InboundClientServiceInterface,
	ouService oupkg.OrganizationUnitServiceInterface,
	i18nService i18nmgt.I18nServiceInterface,
) (ApplicationServiceInterface, declarativeresource.ResourceExporter, error) {
	appService := newApplicationService(
		inboundClient, entityProvider, ouService, i18nService,
	)

	if err := entityService.LoadIndexedAttributes(getAppIndexedAttributes()); err != nil {
		return nil, nil, err
	}

	storeMode := getApplicationStoreMode()
	if storeMode == serverconst.StoreModeComposite || storeMode == serverconst.StoreModeDeclarative {
		if err := entityService.LoadDeclarativeResources(makeAppDeclarativeConfig(appService)); err != nil {
			return nil, nil, err
		}
		if err := inboundClient.LoadDeclarativeResources(
			context.Background(), makeAppInboundConfig(appService)); err != nil {
			return nil, nil, err
		}
	}

	appHandler := newApplicationHandler(appService)
	registerRoutes(mux, appHandler)

	if mcpServer != nil {
		registerMCPTools(mcpServer, appService)
	}

	exporter := newApplicationExporter(appService)
	return appService, exporter, nil
}

func registerRoutes(mux *http.ServeMux, appHandler *applicationHandler) {
	opts1 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("POST /applications",
		appHandler.HandleApplicationPostRequest, opts1))
	mux.HandleFunc(middleware.WithCORS("GET /applications",
		appHandler.HandleApplicationListRequest, opts1))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /applications",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts1))

	opts2 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET", "PUT", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}
	mux.HandleFunc(middleware.WithCORS("GET /applications/{id}",
		appHandler.HandleApplicationGetRequest, opts2))
	mux.HandleFunc(middleware.WithCORS("PUT /applications/{id}",
		appHandler.HandleApplicationPutRequest, opts2))
	mux.HandleFunc(middleware.WithCORS("DELETE /applications/{id}",
		appHandler.HandleApplicationDeleteRequest, opts2))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /applications/",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts2))
}
