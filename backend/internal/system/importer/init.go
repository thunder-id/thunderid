// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import (
	"net/http"

	"github.com/thunder-id/thunderid/internal/agent"
	"github.com/thunder-id/thunderid/internal/application"
	"github.com/thunder-id/thunderid/internal/connection/authzenpdp"
	layoutmgt "github.com/thunder-id/thunderid/internal/design/layout/mgt"
	thememgt "github.com/thunder-id/thunderid/internal/design/theme/mgt"
	"github.com/thunder-id/thunderid/internal/entitytype"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/notification"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/serverconfig"
	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	"github.com/thunder-id/thunderid/internal/system/middleware"
	"github.com/thunder-id/thunderid/internal/user"
	"github.com/thunder-id/thunderid/internal/vc/credential"
	"github.com/thunder-id/thunderid/internal/vc/presentation"
)

// Initialize wires the importer service and registers its HTTP routes.
func Initialize(
	mux *http.ServeMux,
	applicationService application.ApplicationServiceInterface,
	idpService idp.IDPServiceInterface,
	senderService notification.NotificationSenderMgtSvcInterface,
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
	agentService agent.AgentServiceInterface,
	presentationDefinitionService presentation.PresentationDefinitionServiceInterface,
	credentialConfigurationService credential.CredentialConfigurationServiceInterface,
	serverConfigService serverconfig.ServerConfigService,
) ImportServiceInterface {
	importService := newImportService(
		applicationService,
		idpService,
		senderService,
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
		agentService,
		presentationDefinitionService,
		credentialConfigurationService,
		serverConfigService,
		authzenpdp.NewService(authzenpdp.NewStore()),
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
