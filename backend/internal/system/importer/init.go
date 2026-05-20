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

package importer

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/application"
	layoutmgt "github.com/thunder-id/thunderid/internal/design/layout/mgt"
	thememgt "github.com/thunder-id/thunderid/internal/design/theme/mgt"
	"github.com/thunder-id/thunderid/internal/entitytype"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/role"
	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/user"
)

// Initialize wires the importer service and registers its HTTP routes.
func Initialize(
	mux *http.ServeMux,
	applicationService application.ApplicationServiceInterface,
	idpService idp.IDPServiceInterface,
	flowService flowmgt.FlowMgtServiceInterface,
	ouService ou.OrganizationUnitServiceInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
	roleService role.RoleServiceInterface,
	roleAssignmentService role.RoleAssignmentServiceInterface,
	groupService group.GroupServiceInterface,
	resourceService resource.ResourceServiceInterface,
	themeService thememgt.ThemeMgtServiceInterface,
	layoutService layoutmgt.LayoutMgtServiceInterface,
	userService user.UserServiceInterface,
	translationService i18nmgt.I18nServiceInterface,
) ImportServiceInterface {
	importService := newImportService(
		applicationService,
		idpService,
		flowService,
		ouService,
		entityTypeService,
		roleService,
		roleAssignmentService,
		groupService,
		resourceService,
		themeService,
		layoutService,
		userService,
		translationService,
	)
	importHandler := newImportHandler(importService)

	registerRoutes(mux, importHandler)

	return importService
}

func registerRoutes(mux *http.ServeMux, importHandler *importHandler) {
	opts := middleware.CORSOptions{
		AllowedMethods:   []string{"POST"},
		AllowedHeaders:   middleware.DefaultAllowedHeaders,
		AllowCredentials: true,
		MaxAge:           600,
	}

	mux.HandleFunc(middleware.WithCORS("POST /import",
		importHandler.HandleImportRequest, opts))

	mux.HandleFunc(middleware.WithCORS("OPTIONS /import",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))

	mux.HandleFunc(middleware.WithCORS("POST /import/delete",
		importHandler.HandleDeleteImportRequest, opts))
	mux.HandleFunc(middleware.WithCORS("OPTIONS /import/delete",
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}, opts))
}
