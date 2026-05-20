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

package mgt

import (
	"net/http"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/middleware"
)

// Initialize initializes the i18n service and registers its routes.
func Initialize(mux *http.ServeMux) (I18nServiceInterface, declarativeresource.ResourceExporter, error) {
	var store i18nStoreInterface

	storeMode := getI18nStoreMode()
	switch storeMode {
	case serverconst.StoreModeDeclarative:
		fileStore := newFileBasedStore()
		if err := loadDeclarativeResources(fileStore); err != nil {
			return nil, nil, err
		}
		store = fileStore
	case serverconst.StoreModeComposite:
		fileStore := newFileBasedStore()
		if err := loadDeclarativeResources(fileStore); err != nil {
			return nil, nil, err
		}
		store = newCompositeI18nStore(fileStore, newI18nStore())
	default:
		store = newI18nStore()
	}

	service := newI18nService(store)

	handler := newI18nHandler(service)
	registerRoutes(mux, handler)

	exporter := newTranslationExporter(store)
	return service, exporter, nil
}

// registerRoutes registers the routes for i18n management operations.
func registerRoutes(mux *http.ServeMux, handler *i18nHandler) {
	// List languages (public API)
	opts1 := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: false,
		MaxAge:           600,
	}

	mux.HandleFunc(middleware.WithCORS("GET /i18n/languages",
		handler.HandleListLanguages, opts1))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /i18n/languages",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts1))

	// Bulk translation operations
	bulkResolveOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: false,
		MaxAge:           600,
	}

	mux.HandleFunc(middleware.WithCORS("GET /i18n/languages/{language}/translations/resolve",
		handler.HandleResolveTranslationsByLanguage, bulkResolveOpts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /i18n/languages/{language}/translations/resolve",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, bulkResolveOpts))

	bulkEditOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"POST", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	// Shared path for POST and DELETE
	mux.HandleFunc(middleware.WithCORS("POST /i18n/languages/{language}/translations",
		handler.HandleSetOverrideTranslationsByLanguage, bulkEditOpts))
	mux.HandleFunc(middleware.WithCORS("DELETE /i18n/languages/{language}/translations",
		handler.HandleClearOverrideTranslationsByLanguage, bulkEditOpts))

	// Single OPTIONS handler for the shared path
	mux.HandleFunc(middleware.WithCORS("OPTIONS /i18n/languages/{language}/translations",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, bulkEditOpts))

	// Individual translation operations
	singleResolveOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: false,
		MaxAge:           600,
	}

	mux.HandleFunc(middleware.WithCORS(
		"GET /i18n/languages/{language}/translations/ns/{namespace}/keys/{key}/resolve",
		handler.HandleResolveTranslation, singleResolveOpts))
	mux.HandleFunc(middleware.WithCORS(
		"OPTIONS /i18n/languages/{language}/translations/ns/{namespace}/keys/{key}/resolve",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, singleResolveOpts))

	singleEditOpts := middleware.CORSOptions{
		AllowedMethods:   []string{"POST", "DELETE"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	// Shared path for POST and DELETE
	mux.HandleFunc(middleware.WithCORS(
		"POST /i18n/languages/{language}/translations/ns/{namespace}/keys/{key}",
		handler.HandleSetOverrideTranslation, singleEditOpts))
	mux.HandleFunc(middleware.WithCORS(
		"DELETE /i18n/languages/{language}/translations/ns/{namespace}/keys/{key}",
		handler.HandleClearOverrideTranslation, singleEditOpts))

	// Single OPTIONS handler for the shared path
	mux.HandleFunc(middleware.WithCORS(
		"OPTIONS /i18n/languages/{language}/translations/ns/{namespace}/keys/{key}",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, singleEditOpts))
}
