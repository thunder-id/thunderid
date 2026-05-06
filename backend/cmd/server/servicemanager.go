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

// Package managers provides functionality for managing and registering system services.
package main

import (
	"net/http"

	"github.com/asgardeo/thunder/internal/agent"
	"github.com/asgardeo/thunder/internal/application"
	"github.com/asgardeo/thunder/internal/attributecache"
	"github.com/asgardeo/thunder/internal/authn"
	authnAssert "github.com/asgardeo/thunder/internal/authn/assert"
	authncm "github.com/asgardeo/thunder/internal/authn/common"
	authnConsent "github.com/asgardeo/thunder/internal/authn/consent"
	"github.com/asgardeo/thunder/internal/authn/github"
	"github.com/asgardeo/thunder/internal/authn/google"
	authnOAuth "github.com/asgardeo/thunder/internal/authn/oauth"
	authnOIDC "github.com/asgardeo/thunder/internal/authn/oidc"
	"github.com/asgardeo/thunder/internal/authn/otp"
	"github.com/asgardeo/thunder/internal/authn/passkey"
	authnprovidermgr "github.com/asgardeo/thunder/internal/authnprovider/manager"
	"github.com/asgardeo/thunder/internal/authz"
	"github.com/asgardeo/thunder/internal/cert"
	"github.com/asgardeo/thunder/internal/consent"
	layoutmgt "github.com/asgardeo/thunder/internal/design/layout/mgt"
	"github.com/asgardeo/thunder/internal/design/resolve"
	thememgt "github.com/asgardeo/thunder/internal/design/theme/mgt"
	"github.com/asgardeo/thunder/internal/entity"
	"github.com/asgardeo/thunder/internal/entityprovider"
	"github.com/asgardeo/thunder/internal/entitytype"
	flowcore "github.com/asgardeo/thunder/internal/flow/core"
	"github.com/asgardeo/thunder/internal/flow/executor"
	"github.com/asgardeo/thunder/internal/flow/flowexec"
	"github.com/asgardeo/thunder/internal/flow/flowmeta"
	flowmgt "github.com/asgardeo/thunder/internal/flow/mgt"
	"github.com/asgardeo/thunder/internal/group"
	"github.com/asgardeo/thunder/internal/idp"
	"github.com/asgardeo/thunder/internal/inboundclient"
	"github.com/asgardeo/thunder/internal/notification"
	"github.com/asgardeo/thunder/internal/oauth"
	"github.com/asgardeo/thunder/internal/ou"
	"github.com/asgardeo/thunder/internal/resource"
	"github.com/asgardeo/thunder/internal/role"
	"github.com/asgardeo/thunder/internal/system/crypto/hash"
	"github.com/asgardeo/thunder/internal/system/crypto/pki"
	dbprovider "github.com/asgardeo/thunder/internal/system/database/provider"
	declarativeresource "github.com/asgardeo/thunder/internal/system/declarative_resource"
	"github.com/asgardeo/thunder/internal/system/email"
	"github.com/asgardeo/thunder/internal/system/export"
	healthcheckservice "github.com/asgardeo/thunder/internal/system/healthcheck/service"
	i18nmgt "github.com/asgardeo/thunder/internal/system/i18n/mgt"
	"github.com/asgardeo/thunder/internal/system/importer"
	"github.com/asgardeo/thunder/internal/system/jose"
	"github.com/asgardeo/thunder/internal/system/jose/jwt"
	"github.com/asgardeo/thunder/internal/system/log"
	"github.com/asgardeo/thunder/internal/system/mcp"
	"github.com/asgardeo/thunder/internal/system/observability"
	"github.com/asgardeo/thunder/internal/system/services"
	"github.com/asgardeo/thunder/internal/system/sysauthz"
	"github.com/asgardeo/thunder/internal/system/template"
	"github.com/asgardeo/thunder/internal/user"
)

// observabilitySvc is the observability service instance. This is used for graceful shutdown.
var observabilitySvc observability.ObservabilityServiceInterface

// registerServices registers all the services with the provided HTTP multiplexer.
func registerServices(mux *http.ServeMux) jwt.JWTServiceInterface {
	logger := log.GetLogger()

	// Load the server's private key for signing JWTs.
	pkiService, err := pki.Initialize()
	if err != nil {
		logger.Fatal("Failed to initialize certificate service", log.Error(err))
	}

	jwtService, jweService, err := jose.Initialize(pkiService)
	if err != nil {
		logger.Fatal("Failed to initialize JOSE services", log.Error(err))
	}

	observabilitySvc = observability.Initialize()

	// List to collect exporters from each package
	var exporters []declarativeresource.ResourceExporter

	// Initialize i18n service for internationalization support.
	i18nService, i18nExporter, err := i18nmgt.Initialize(mux)
	if err != nil {
		logger.Fatal("Failed to initialize i18n service", log.Error(err))
	}
	// Add to exporters list (must be done after initializing list)
	exporters = append(exporters, i18nExporter)

	ouAuthzService, err := sysauthz.Initialize()
	if err != nil {
		logger.Fatal("Failed to initialize system authorization service", log.Error(err))
	}

	ouService, ouHierarchyResolver, ouExporter, err := ou.Initialize(mux, ouAuthzService)
	if err != nil {
		logger.Fatal("Failed to initialize OrganizationUnitService", log.Error(err))
	}
	exporters = append(exporters, ouExporter)

	// Complete the two-phase initialization: inject the OU hierarchy resolver into the
	// authz service now that the ou package is ready. This breaks the import-cycle that
	// would arise if sysauthz were to directly import the ou package.
	ouAuthzService.SetOUHierarchyResolver(ouHierarchyResolver)

	hashService, err := hash.Initialize()
	if err != nil {
		logger.Fatal("Failed to initialize HashService", log.Error(err))
	}

	// Initialize consent service
	consentService := consent.Initialize()

	// Initialize user type service
	entityTypeService, entityTypeExporter, err := entitytype.Initialize(
		mux, ouService, ouAuthzService, consentService)
	if err != nil {
		logger.Fatal("Failed to initialize EntityTypeService", log.Error(err))
	}
	exporters = append(exporters, entityTypeExporter)

	// Initialize entity service
	entityService, err := entity.Initialize(hashService, entityTypeService, ouService)
	if err != nil {
		logger.Fatal("Failed to initialize EntityService", log.Error(err))
	}

	// Initialize entity provider
	entityProvider := entityprovider.InitializeEntityProvider(entityService)

	userService, ouUserResolver, userExporter, err := user.Initialize(
		mux, entityService, ouService, entityTypeService, ouAuthzService,
	)
	if err != nil {
		logger.Fatal("Failed to initialize UserService", log.Error(err))
	}
	exporters = append(exporters, userExporter)

	groupService, ouGroupResolver, err := group.Initialize(
		mux, dbprovider.GetDBProvider(), ouService, entityService, entityTypeService, ouAuthzService,
	)
	if err != nil {
		logger.Fatal("Failed to initialize GroupService", log.Error(err))
	}

	// Two-phase initialization: inject user/group resolvers into OU service.
	ouService.SetOUUserResolver(ouUserResolver)
	ouService.SetOUGroupResolver(ouGroupResolver)

	resourceService, resourceExporter, err := resource.Initialize(mux, ouService)
	if err != nil {
		logger.Fatal("Failed to initialize Resource Service", log.Error(err))
	}
	exporters = append(exporters, resourceExporter)
	roleService, roleExporter, err := role.Initialize(
		mux, entityService, groupService, ouService, resourceService, entityTypeService,
	)
	if err != nil {
		logger.Fatal("Failed to initialize RoleService", log.Error(err))
	}
	exporters = append(exporters, roleExporter)
	authZService := authz.Initialize(roleService)

	idpService, idpExporter, err := idp.Initialize(mux)
	if err != nil {
		logger.Fatal("Failed to initialize IDPService", log.Error(err))
	}
	exporters = append(exporters, idpExporter)

	templateService, err := template.Initialize()
	if err != nil {
		logger.Fatal("Failed to initialize template service", log.Error(err))
	}

	_, otpService, notifSenderSvc, notificationExporter, err := notification.Initialize(
		mux, jwtService, templateService)
	if err != nil {
		logger.Fatal("Failed to initialize NotificationService", log.Error(err))
	}
	exporters = append(exporters, notificationExporter)

	// Initialize MCP server
	mcpServer := mcp.Initialize(mux, jwtService)

	// Initialize passkey service
	passkeyService := passkey.Initialize(entityService)

	// Initialize otp core service
	otpCoreService := otp.Initialize(otpService, entityProvider)

	// Initialize federated authentication services.
	oauthAuthnService := authnOAuth.Initialize(idpService, entityProvider)
	oidcAuthnService := authnOIDC.Initialize(oauthAuthnService, jwtService)
	googleAuthnService := google.Initialize(oidcAuthnService, jwtService)
	githubAuthnService := github.Initialize(oauthAuthnService)

	federatedAuths := map[idp.IDPType]authncm.FederatedAuthenticator{
		idp.IDPTypeOAuth:  oauthAuthnService,
		idp.IDPTypeOIDC:   oidcAuthnService,
		idp.IDPTypeGoogle: googleAuthnService,
		idp.IDPTypeGitHub: githubAuthnService,
	}

	// Initialize authn provider
	authnProvider := authnprovidermgr.InitializeAuthnProviderManager(entityService, passkeyService, otpCoreService,
		federatedAuths)

	// Initialize authentication services.
	authAssertGen := authnAssert.Initialize(authnProvider)
	consentEnforcer := authnConsent.Initialize(consentService, jwtService)

	authn.Initialize(mux, mcpServer, idpService, jwtService, authnProvider, authAssertGen, passkeyService,
		otpCoreService, oauthAuthnService, oidcAuthnService, googleAuthnService, githubAuthnService)

	attributeCacheService := attributecache.Initialize()

	// Initialize flow and executor services.
	flowFactory, graphCache := flowcore.Initialize()
	var emailClient email.EmailClientInterface
	emailClient, err = email.Initialize()
	if err != nil {
		logger.Debug("Email client not configured. "+
			"EmailExecutor will be registered but will not send emails.", log.Error(err))
		emailClient = nil
	}
	execRegistry := executor.Initialize(flowFactory, ouService, idpService, notifSenderSvc, jwtService, authAssertGen,
		consentEnforcer, authnProvider, otpCoreService, passkeyService, authZService, entityTypeService,
		observabilitySvc, groupService, roleService, entityProvider, attributeCacheService, emailClient,
		templateService, oauthAuthnService, oidcAuthnService, githubAuthnService, googleAuthnService)

	flowMgtService, flowMgtExporter, err := flowmgt.Initialize(mux, mcpServer, flowFactory, execRegistry, graphCache)
	if err != nil {
		logger.Fatal("Failed to initialize FlowMgtService", log.Error(err))
	}
	exporters = append(exporters, flowMgtExporter)
	certservice, err := cert.Initialize(dbprovider.GetDBProvider())
	if err != nil {
		logger.Fatal("Failed to initialize CertificateService", log.Error(err))
	}

	// Initialize theme and layout services
	themeMgtService, themeExporter, err := thememgt.Initialize(mux)
	if err != nil {
		logger.Fatal("Failed to initialize ThemeMgtService", log.Error(err))
	}
	exporters = append(exporters, themeExporter)

	layoutMgtService, layoutExporter, err := layoutmgt.Initialize(mux)
	if err != nil {
		logger.Fatal("Failed to initialize LayoutMgtService", log.Error(err))
	}
	exporters = append(exporters, layoutExporter)

	inboundClientService, err := inboundclient.Initialize(
		certservice, entityProvider,
		themeMgtService, layoutMgtService, flowMgtService, entityTypeService, consentService)
	if err != nil {
		logger.Fatal("Failed to initialize InboundClientService", log.Error(err))
	}

	// TODO: Remove entityService dependency after finalizing declarative resource loading pattern
	applicationService, applicationExporter, err := application.Initialize(
		mux, mcpServer, entityProvider, entityService, inboundClientService, ouService, i18nService)
	if err != nil {
		logger.Fatal("Failed to initialize ApplicationService", log.Error(err))
	}
	exporters = append(exporters, applicationExporter)

	if _, err := agent.Initialize(mux, entityService, inboundClientService, ouService); err != nil {
		logger.Fatal("Failed to initialize AgentService", log.Error(err))
	}

	// Initialize design resolve service for theme and layout resolution
	designResolveService := resolve.Initialize(mux, themeMgtService, layoutMgtService, applicationService)

	// Initialize flow metadata service
	_ = flowmeta.Initialize(mux, applicationService, ouService, designResolveService, i18nService)

	// Initialize export service with collected exporters
	_ = export.Initialize(mux, exporters)

	// Initialize import service
	_ = importer.Initialize(
		mux,
		applicationService,
		idpService,
		flowMgtService,
		ouService,
		entityTypeService,
		roleService,
		resourceService,
		themeMgtService,
		layoutMgtService,
		userService,
		i18nService,
	)

	flowExecService, err := flowexec.Initialize(mux, flowMgtService, applicationService, execRegistry,
		observabilitySvc)
	if err != nil {
		logger.Fatal("Failed to initialize flow execution service", log.Error(err))
	}

	// Initialize OAuth services.
	err = oauth.Initialize(mux, applicationService, inboundClientService, authnProvider, jwtService, jweService,
		flowExecService, observabilitySvc, pkiService, ouService, attributeCacheService, authZService, entityProvider,
		resourceService, i18nService)
	if err != nil {
		logger.Fatal("Failed to initialize OAuth services", log.Error(err))
	}

	// Register the health service.
	healthSvc := healthcheckservice.Initialize(dbprovider.GetDBProvider(), dbprovider.GetRedisProvider())
	services.NewHealthCheckService(mux, healthSvc)

	return jwtService
}

// unregisterServices unregisters all services that require cleanup during shutdown.
func unregisterServices() {
	observabilitySvc.Shutdown()
}
