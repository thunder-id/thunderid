/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/actorprovider"
	"github.com/thunder-id/thunderid/internal/agent"
	"github.com/thunder-id/thunderid/internal/application"
	"github.com/thunder-id/thunderid/internal/attestation"
	"github.com/thunder-id/thunderid/internal/attributecache"
	"github.com/thunder-id/thunderid/internal/authn"
	authnAssert "github.com/thunder-id/thunderid/internal/authn/assert"
	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	authnConsent "github.com/thunder-id/thunderid/internal/authn/consent"
	"github.com/thunder-id/thunderid/internal/authn/github"
	"github.com/thunder-id/thunderid/internal/authn/google"
	"github.com/thunder-id/thunderid/internal/authn/magiclink"
	authnOAuth "github.com/thunder-id/thunderid/internal/authn/oauth"
	authnOIDC "github.com/thunder-id/thunderid/internal/authn/oidc"
	"github.com/thunder-id/thunderid/internal/authn/openid4vp"
	"github.com/thunder-id/thunderid/internal/authn/otp"
	"github.com/thunder-id/thunderid/internal/authn/passkey"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/authz"
	"github.com/thunder-id/thunderid/internal/authzen"
	"github.com/thunder-id/thunderid/internal/cert"
	"github.com/thunder-id/thunderid/internal/connection"
	"github.com/thunder-id/thunderid/internal/consent"
	layoutmgt "github.com/thunder-id/thunderid/internal/design/layout/mgt"
	"github.com/thunder-id/thunderid/internal/design/resolve"
	thememgt "github.com/thunder-id/thunderid/internal/design/theme/mgt"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
	flowcore "github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/executor"
	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	"github.com/thunder-id/thunderid/internal/flow/flowmeta"
	"github.com/thunder-id/thunderid/internal/flow/graphbuilder"
	"github.com/thunder-id/thunderid/internal/flow/interceptor"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	flowsession "github.com/thunder-id/thunderid/internal/flow/session"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	"github.com/thunder-id/thunderid/internal/notification"
	"github.com/thunder-id/thunderid/internal/oauth"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dcr"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jti"
	"github.com/thunder-id/thunderid/internal/openid4vci"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/runtimestore"
	"github.com/thunder-id/thunderid/internal/serverconfig"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cors"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	dbprovider "github.com/thunder-id/thunderid/internal/system/database/provider"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
	"github.com/thunder-id/thunderid/internal/system/email"
	"github.com/thunder-id/thunderid/internal/system/export"
	healthcheckservice "github.com/thunder-id/thunderid/internal/system/healthcheck/service"
	i18nmgt "github.com/thunder-id/thunderid/internal/system/i18n/mgt"
	"github.com/thunder-id/thunderid/internal/system/importer"
	"github.com/thunder-id/thunderid/internal/system/jose"
	joseconfig "github.com/thunder-id/thunderid/internal/system/jose/config"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/kmprovider"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/defaultkm/pki"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/mcp"
	"github.com/thunder-id/thunderid/internal/system/observability"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/services"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/internal/system/template"
	"github.com/thunder-id/thunderid/internal/user"
	"github.com/thunder-id/thunderid/internal/vc/credential"
	"github.com/thunder-id/thunderid/internal/vc/presentation"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// observabilitySvc is the observability service instance. This is used for graceful shutdown.
var observabilitySvc observability.ObservabilityServiceInterface

// registerServices registers all the services with the provided HTTP multiplexer.
// It also returns the import service so the bootstrap subcommand can create default
// resources in-process through the same service instances.
func registerServices(mux *http.ServeMux, cacheManager cache.CacheManagerInterface) (
	jwt.JWTServiceInterface, kmprovider.RuntimeCryptoProvider, importer.ImportServiceInterface) {
	logger := log.GetLogger()

	// Service registration runs during application startup, outside any request.
	ctx := context.Background()

	// Load the server's private key for signing JWTs.
	pkiService, err := pki.Initialize()
	fatalOnError(ctx, logger, err, "Failed to initialize certificate service")

	runtimeCryptoSvc, configCryptoSvc, err := kmprovider.Initialize(pkiService)
	fatalOnError(ctx, logger, err, "Failed to initialize key manager provider")

	runtime := config.GetServerRuntime()
	joseCfg := joseconfig.Config{
		Issuer:         runtime.Config.JWT.Issuer,
		ValidityPeriod: runtime.Config.JWT.ValidityPeriod,
		Audience:       runtime.Config.JWT.Audience,
		PreferredKeyID: runtime.Config.JWT.PreferredKeyID,
		Leeway:         runtime.Config.JWT.Leeway,
		JWKSCacheTTL:   time.Duration(runtime.Config.Server.SecurityConfig.JWKSCacheTTL) * time.Second,
	}
	jwtService, jweService, err := jose.Initialize(runtimeCryptoSvc, joseCfg)
	fatalOnError(ctx, logger, err, "Failed to initialize JOSE services")

	observabilitySvc = observability.Initialize(config.GetServerRuntime().Config.Observability)

	// Initialize MCP server early so packages initializing below can register tools.
	mcpServer := mcp.Initialize(mux, jwtService)

	// List to collect exporters from each package
	var exporters []declarativeresource.ResourceExporter

	// Initialize i18n service for internationalization support.
	i18nService, i18nExporter, err := i18nmgt.Initialize(mux, config.GetServerRuntime().Config.Translation)
	fatalOnError(ctx, logger, err, "Failed to initialize i18n service")
	// Add to exporters list (must be done after initializing list)
	exporters = append(exporters, i18nExporter)

	ouAuthzService, err := sysauthz.Initialize()
	fatalOnError(ctx, logger, err, "Failed to initialize system authorization service")

	ouService, ouHierarchyResolver, ouExporter, err := ou.Initialize(mux, mcpServer, cacheManager, ouAuthzService)
	fatalOnError(ctx, logger, err, "Failed to initialize OrganizationUnitService")
	exporters = append(exporters, ouExporter)

	// Complete the two-phase initialization: inject the OU hierarchy resolver into the
	// authz service now that the ou package is ready. This breaks the import-cycle that
	// would arise if sysauthz were to directly import the ou package.
	ouAuthzService.SetOUHierarchyResolver(ouHierarchyResolver)

	hashCfg, err := buildHashConfig()
	fatalOnError(ctx, logger, err, "Failed to build HashService config")
	hashService, err := cryptolib.Initialize(hashCfg)
	fatalOnError(ctx, logger, err, "Failed to initialize HashService")

	// Initialize user type service
	entityTypeService, entityTypeExporter, err := entitytype.Initialize(
		mux, mcpServer, cacheManager, ouService, ouAuthzService)
	fatalOnError(ctx, logger, err, "Failed to initialize EntityTypeService")
	exporters = append(exporters, entityTypeExporter)

	// Initialize entity service
	entityService, err := entity.Initialize(cacheManager, hashService, entityTypeService, ouService)
	fatalOnError(ctx, logger, err, "Failed to initialize EntityService")

	// Initialize entity provider
	entityProvider := entityprovider.InitializeEntityProvider(entityService)

	userService, ouUserResolver, userExporter, err := user.Initialize(
		mux, entityService, ouService, entityTypeService, ouAuthzService,
	)
	fatalOnError(ctx, logger, err, "Failed to initialize UserService")
	exporters = append(exporters, userExporter)

	groupService, ouGroupResolver, groupExporter, err := group.Initialize(
		mux, dbprovider.GetDBProvider(), ouService, entityService, entityTypeService, ouAuthzService,
	)
	fatalOnError(ctx, logger, err, "Failed to initialize GroupService")
	exporters = append(exporters, groupExporter)

	resourceService, resourceExporter, err := resource.Initialize(mux, ouService)
	fatalOnError(ctx, logger, err, "Failed to initialize Resource Service")
	exporters = append(exporters, resourceExporter)

	roleService, roleAssignmentService, ouRoleResolver, roleExporter, err := role.Initialize(
		mux, entityService, groupService, ouService, resourceService, entityTypeService,
	)
	fatalOnError(ctx, logger, err, "Failed to initialize RoleService")
	exporters = append(exporters, roleExporter)

	// Two-phase initialization: inject user/group/role resolvers into OU service.
	ouService.SetOUUserResolver(ouUserResolver)
	ouService.SetOUGroupResolver(ouGroupResolver)
	ouService.SetOURoleResolver(ouRoleResolver)

	authZService := authz.Initialize(roleService)

	idpService, err := idp.Initialize(cacheManager, entityTypeService)
	fatalOnError(ctx, logger, err, "Failed to initialize IDPService")

	templateService, err := template.Initialize()
	fatalOnError(ctx, logger, err, "Failed to initialize template service")

	notifSenderMgtSvc, notifOTPService, notifSenderSvc, err := notification.Initialize(jwtService)
	fatalOnError(ctx, logger, err, "Failed to initialize NotificationService")

	// Register the /connections API as a thin layer over the identity-provider and
	// notification-sender services.
	connectionExporter, err := connection.Initialize(mux, idpService, notifSenderMgtSvc)
	fatalOnError(ctx, logger, err, "Failed to initialize connection declarative resources")
	exporters = append(exporters, connectionExporter)

	// Initialize passkey service
	passkeyService := passkey.Initialize(entityService)

	// Initialize magic link service
	magicLinkService := magiclink.Initialize(jwtService)

	// Initialize otp core service
	otpCoreService := otp.Initialize(notifOTPService)

	// Initialize federated authentication services.
	oauthAuthnService := authnOAuth.Initialize(idpService, entityProvider)
	oidcAuthnService := authnOIDC.Initialize(oauthAuthnService, jwtService)
	googleAuthnService := google.Initialize(oidcAuthnService, jwtService)
	githubAuthnService := github.Initialize(oauthAuthnService)

	federatedAuths := map[providers.IDPType]authncm.FederatedAuthenticator{
		providers.IDPTypeOAuth:  oauthAuthnService,
		providers.IDPTypeOIDC:   oidcAuthnService,
		providers.IDPTypeGoogle: googleAuthnService,
		providers.IDPTypeGitHub: githubAuthnService,
	}

	// Shared DPoP verifier (and its JTI replay cache) so OAuth and OpenID4VCI
	// share JTI replay protection.
	oauthCfg := oauthconfig.FromServerRuntime()
	dpopVerifier := dpop.Initialize(oauthCfg, jti.Initialize(oauthCfg))

	runtimeStoreProvider, transactioner, err := runtimestore.Initialize(runtime.Config.Database.RuntimeTransient.Type,
		runtime.Config.Server.Identifier)
	fatalOnError(ctx, logger, err, "Failed to initialize runtime store")

	openid4vpSvc, openid4vpDefSvc, openid4vciCredSvc, exporters :=
		initializeVCServices(ctx, logger, mux, runtimeCryptoSvc, configCryptoSvc, jwtService, userService,
			ouService, dpopVerifier, runtimeStoreProvider, exporters)

	// Initialize authn provider
	authnProvider := authnprovidermgr.InitializeAuthnProviderManager(entityService, passkeyService, otpCoreService,
		magicLinkService, openid4vpSvc, federatedAuths)

	// Initialize authentication services.
	authAssertGen := authnAssert.Initialize()
	consentEnforcer := authnConsent.Initialize(jwtService)

	_, directAuthGuard := authn.Initialize(mux, mcpServer, idpService, jwtService, authnProvider, authAssertGen,
		passkeyService, otpCoreService, notifSenderSvc, templateService, magicLinkService, oauthAuthnService,
		oidcAuthnService, googleAuthnService, githubAuthnService,
		runtime.Config.Server.SecurityConfig.DirectAuthSecret)

	// AuthZEN access-evaluation endpoints are Direct API endpoints, so they reuse the Direct Auth
	// guard created by the authn service.
	authzen.Initialize(mux, authZService, entityProvider, resourceService, directAuthGuard)

	attributeCacheService := attributecache.Initialize(runtimeStoreProvider)

	emailClient := initEmailClient(ctx, logger)

	// Initialize server-wide configuration after its handler dependencies.
	serverConfigHandlers := map[serverconfig.ConfigName]serverconfig.ServerConfigHandlerInterface{
		serverconfig.ConfigNameCORS:                  cors.OriginHandler{},
		serverconfig.ConfigNameDefaultResourceServer: resource.NewDefaultResourceServerConfigHandler(resourceService),
		serverconfig.ConfigNameSession:               flowsession.ConfigHandler{},
	}
	serverConfigService, serverConfigExporter, err := serverconfig.Initialize(mux, cacheManager, serverConfigHandlers)
	fatalOnError(ctx, logger, err, "Failed to initialize server config service")
	exporters = append(exporters, serverConfigExporter)

	// CORS origins come from the server-config cors section.
	cors.InitializeDynamicMatcher(serverConfigService)

	flowConfig := flowconfig.FromServerRuntime()
	sessionService, sessionCfg := initSessionService(ctx, serverConfigService, runtime.Config.Server.Identifier, logger)
	flowConfig.Session = sessionCfg
	flowFactory, execRegistry, interceptorRegistry, graphBuilder := initializeFlowCoreAndExecutor(ctx, logger,
		cacheManager, executor.ExecutorDependencies{
			OUService:             ouService,
			IDPService:            idpService,
			NotifSenderSvc:        notifSenderSvc,
			JWTService:            jwtService,
			AuthAssertGen:         authAssertGen,
			ConsentEnforcer:       consentEnforcer,
			AuthnProvider:         authnProvider,
			OTPService:            otpCoreService,
			PasskeyService:        passkeyService,
			MagicLinkService:      magicLinkService,
			AuthZService:          authZService,
			EntityTypeService:     entityTypeService,
			GroupService:          groupService,
			RoleService:           roleService,
			RoleAssignmentService: roleAssignmentService,
			EntityProvider:        entityProvider,
			AttributeCacheSvc:     attributeCacheService,
			EmailClient:           emailClient,
			TemplateService:       templateService,
			OAuthSvc:              oauthAuthnService,
			OIDCSvc:               oidcAuthnService,
			GithubSvc:             githubAuthnService,
			GoogleSvc:             googleAuthnService,
			OpenID4VPVerifierSvc:  openid4vpSvc,
			SessionService:        sessionService,
		},
		interceptor.InterceptorDependencies{},
		flowConfig,
	)

	flowMgtService, flowMgtExporter, err := flowmgt.Initialize(
		mux, mcpServer, cacheManager, flowFactory, execRegistry, interceptorRegistry, graphBuilder)
	fatalOnError(ctx, logger, err, "Failed to initialize FlowMgtService")

	exporters = append(exporters, flowMgtExporter)
	certservice, err := cert.Initialize(cacheManager, dbprovider.GetDBProvider())
	fatalOnError(ctx, logger, err, "Failed to initialize CertificateService")

	// Initialize theme and layout services
	themeMgtService, themeExporter, err := thememgt.Initialize(mux, mcpServer)
	fatalOnError(ctx, logger, err, "Failed to initialize ThemeMgtService")
	exporters = append(exporters, themeExporter)

	layoutMgtService, layoutExporter, err := layoutmgt.Initialize(mux)
	fatalOnError(ctx, logger, err, "Failed to initialize LayoutMgtService")
	exporters = append(exporters, layoutExporter)

	inboundClientService, err := inboundclient.Initialize(
		cacheManager, certservice, entityProvider,
		themeMgtService, layoutMgtService, flowMgtService, entityTypeService)
	fatalOnError(ctx, logger, err, "Failed to initialize InboundClientService")

	// Inject the consent service into the consent enforcer. It is wired here rather than at enforcer
	// construction because it depends on the inbound client service, which is only available after the
	// flow services (which themselves depend on the enforcer) are initialized.
	consentEnforcer.SetConsentService(initConsentService(ctx, logger, inboundClientService))

	// TODO: Remove entityService dependency after finalizing declarative resource loading pattern
	applicationService, applicationExporter, err := application.Initialize(
		mux, mcpServer, entityProvider, entityService, inboundClientService, ouService, i18nService,
		runtimeCryptoSvc)
	fatalOnError(ctx, logger, err, "Failed to initialize ApplicationService")
	exporters = append(exporters, applicationExporter)

	agentService, agentExporter, err := agent.Initialize(mux, entityService, inboundClientService, ouService,
		roleService)
	fatalOnError(ctx, logger, err, "Failed to initialize AgentService")
	exporters = append(exporters, agentExporter)

	// Wire the dependency registry into the consuming services (two-phase init to avoid cyclic
	// imports). flowMgtService is both a consumer and a provider: it reports which flows reference an
	// identity provider or notification sender.
	registerDependencyRegistry(dependencyConsumers{
		theme:       themeMgtService,
		layout:      layoutMgtService,
		flow:        flowMgtService,
		user:        userService,
		idp:         idpService,
		notifSender: notifSenderMgtSvc,
		application: applicationService,
		agent:       agentService,
		group:       groupService,
		ou:          ouService,
		resource:    resourceService,
	}, applicationService, agentService, flowMgtService, roleAssignmentService, groupService,
		ouService, ouUserResolver, ouGroupResolver, resourceService)

	// Initialize design resolve service for theme and layout resolution
	designResolveService := resolve.Initialize(mux, themeMgtService, layoutMgtService, applicationService)

	actorProvider := actorprovider.Initialize(inboundClientService, entityProvider, authnProvider)

	// Initialize flow metadata service
	_ = flowmeta.Initialize(mux, actorProvider, ouService, designResolveService, i18nService)

	// Initialize export service with collected exporters
	_ = export.Initialize(mux, exporters)

	// Initialize import service
	importService := importer.Initialize(
		mux,
		applicationService,
		idpService,
		notifSenderMgtSvc,
		flowMgtService,
		ouService,
		entityTypeService,
		roleService,
		roleAssignmentService,
		groupService,
		resourceService,
		themeMgtService,
		layoutMgtService,
		userService,
		i18nService,
		agentService,
		openid4vpDefSvc,
		openid4vciCredSvc,
		serverConfigService,
	)

	attestationProvider := initAttestationProvider(ctx, logger, runtimeCryptoSvc)
	flowExecService, err := flowexec.Initialize(mux, flowMgtService, actorProvider,
		execRegistry, interceptorRegistry, observabilitySvc, runtimeCryptoSvc, attestationProvider,
		graphBuilder, runtimeStoreProvider, transactioner, flowConfig)
	fatalOnError(ctx, logger, err, "Failed to initialize flow execution service")

	// Initialize OAuth services.
	err = oauth.Initialize(mux, actorProvider, authnProvider, jwtService, jweService,
		flowExecService, observabilitySvc, runtimeCryptoSvc, ouService, attributeCacheService, authZService,
		resourceService, serverConfigService, i18nService, idpService, dpopVerifier,
		runtimeStoreProvider, oauthCfg)
	fatalOnError(ctx, logger, err, "Failed to initialize OAuth services")

	// Register OAuth2 DCR service.
	err = dcr.Initialize(mux, applicationService, ouService, i18nService, oauthCfg)
	fatalOnError(ctx, logger, err, "Failed to initialize OAuth2 DCR service")

	// Register the health service.
	healthSvc := healthcheckservice.Initialize(dbprovider.GetDBProvider(), dbprovider.GetRedisProvider())
	services.NewHealthCheckService(mux, healthSvc)

	return jwtService, runtimeCryptoSvc, importService
}

// initAttestationProvider initializes the platform attestation provider, terminating server startup
// on failure rather than running with a non-functional verifier.
func initAttestationProvider(ctx context.Context, logger *log.Logger,
	cryptoSvc kmprovider.RuntimeCryptoProvider) providers.AttestationProvider {
	attestationProvider, err := attestation.Initialize(cryptoSvc)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize attestation provider", log.Error(err))
	}
	return attestationProvider
}

// dependencyConsumers groups the services that check the dependency registry before deleting their
// own resources.
type dependencyConsumers struct {
	theme       thememgt.ThemeMgtServiceInterface
	layout      layoutmgt.LayoutMgtServiceInterface
	flow        flowmgt.FlowMgtServiceInterface
	user        user.UserServiceInterface
	idp         idp.IDPServiceInterface
	notifSender notification.NotificationSenderMgtSvcInterface
	application application.ApplicationServiceInterface
	agent       agent.AgentServiceInterface
	group       group.GroupServiceInterface
	ou          ou.ConfigurableOUService
	resource    resource.ResourceServiceInterface
}

// registerDependencyRegistry builds the dependency registry from the given providers and wires it
// into the consuming services.
func registerDependencyRegistry(consumers dependencyConsumers, providers ...resourcedependency.Provider) {
	registry := resourcedependency.Initialize(providers...)
	consumers.theme.SetDependencyRegistry(registry)
	consumers.layout.SetDependencyRegistry(registry)
	consumers.flow.SetDependencyRegistry(registry)
	consumers.user.SetDependencyRegistry(registry)
	consumers.idp.SetDependencyRegistry(registry)
	consumers.notifSender.SetDependencyRegistry(registry)
	consumers.application.SetDependencyRegistry(registry)
	consumers.agent.SetDependencyRegistry(registry)
	consumers.group.SetDependencyRegistry(registry)
	consumers.ou.SetDependencyRegistry(registry)
	consumers.resource.SetDependencyRegistry(registry)
}

// unregisterServices unregisters all services that require cleanup during shutdown.
func unregisterServices() {
	observabilitySvc.Shutdown()
}

// initSessionService reads the effective SSO session configuration from the server-config section and
// builds the session service, returning both so the caller can thread the config into flowexec too.
func initSessionService(ctx context.Context, svc serverconfig.ServerConfigService, deploymentID string,
	logger *log.Logger) (flowsession.Service, flowsession.Config) {
	cfg := readSessionConfig(ctx, svc, logger)
	sessionService, err := flowsession.Initialize(dbprovider.GetDBProvider(), deploymentID,
		flowsession.NewTimeouts(cfg.IdleTimeoutSeconds, cfg.AbsoluteTimeoutSeconds))
	fatalOnError(ctx, logger, err, "Failed to initialize SSO session service")
	return sessionService, cfg
}

// readSessionConfig reads the effective SSO session lifetime configuration from the server-config
// "session" section. An unset section resolves to the zero Config, which NewTimeouts turns into the
// built-in defaults; a read error is non-fatal for the same reason, so it logs and falls back.
func readSessionConfig(ctx context.Context, svc serverconfig.ServerConfigService,
	logger *log.Logger) flowsession.Config {
	merged, svcErr := svc.GetMergedConfig(ctx, string(serverconfig.ConfigNameSession))
	if svcErr != nil {
		logger.Warn(ctx, "Failed to read session server config; using default timeouts",
			log.String("code", svcErr.Code))
		return flowsession.Config{}
	}
	cfg, _ := merged.(flowsession.Config)
	return cfg
}

// initConsentService initializes the consent service backed by the inbound client service, which
// satisfies consent.InboundClientProvider directly.
func initConsentService(ctx context.Context, logger *log.Logger,
	inboundClientService inboundclient.InboundClientServiceInterface) consent.ConsentServiceInterface {
	consentService, err := consent.Initialize(inboundClientService)
	fatalOnError(ctx, logger, err, "Failed to initialize consent service")
	return consentService
}

// fatalOnError logs msg and exits the process if err is non-nil.
func fatalOnError(ctx context.Context, logger *log.Logger, err error, msg string) {
	if err != nil {
		logger.Fatal(ctx, msg, log.Error(err))
	}
}

// initEmailClient initializes the email client, returning nil if not configured.
func initEmailClient(ctx context.Context, logger *log.Logger) email.EmailClientInterface {
	client, err := email.Initialize()
	if err != nil {
		logger.Debug(ctx, "Email client not configured. "+
			"EmailExecutor will be registered but will not send emails.", log.Error(err))
		return nil
	}
	return client
}

// initializeFlowCoreAndExecutor initializes the flow core and executor services.
func initializeFlowCoreAndExecutor(
	ctx context.Context,
	logger *log.Logger,
	cacheManager cache.CacheManagerInterface,
	execDeps executor.ExecutorDependencies,
	interceptorDeps interceptor.InterceptorDependencies,
	flowConfig flowconfig.Config,
) (flowcore.FlowFactoryInterface, executor.ExecutorRegistryInterface,
	interceptor.InterceptorRegistryInterface, graphbuilder.GraphBuilderInterface) {
	// Initialize flow core services.
	flowFactory, graphCache := flowcore.Initialize(cacheManager)
	execDeps.FlowFactory = flowFactory
	interceptorDeps.FlowFactory = flowFactory

	// Initialize flow executor registry
	execRegistry, err := executor.Initialize(execDeps, flowConfig.Flow)
	fatalOnError(ctx, logger, err, "Failed to register flow executors")
	interceptorRegistry, err := interceptor.Initialize(interceptorDeps, flowConfig.Flow)
	fatalOnError(ctx, logger, err, "Failed to initialize Interceptor registry")

	graphBuilder := graphbuilder.Initialize(flowFactory, execRegistry, interceptorRegistry, graphCache)

	return flowFactory, execRegistry, interceptorRegistry, graphBuilder
}

// initializeVCServices initializes the OpenID4VP verifier and OpenID4VCI issuer services,
// appending their declarative-resource exporters to exporters.
func initializeVCServices(
	ctx context.Context, logger *log.Logger, mux *http.ServeMux,
	runtimeCrypto kmprovider.RuntimeCryptoProvider, configCrypto kmprovider.ConfigCryptoProvider,
	jwtService jwt.JWTServiceInterface, userService user.UserServiceInterface,
	ouService ou.OrganizationUnitServiceInterface,
	dpopVerifier dpop.VerifierInterface,
	runtimeStoreProvider providers.RuntimeStoreProvider,
	exporters []declarativeresource.ResourceExporter,
) (openid4vp.OpenID4VPServiceInterface, presentation.PresentationDefinitionServiceInterface,
	credential.CredentialConfigurationServiceInterface, []declarativeresource.ResourceExporter) {
	openid4vpDefSvc, vpDefExp, err := presentation.Initialize(mux, ouService)
	fatalOnError(ctx, logger, err, "Failed to initialize presentation definition service")
	if vpDefExp != nil {
		exporters = append(exporters, vpDefExp)
	}

	openid4vpSvc, err := openid4vp.Initialize(mux, runtimeCrypto, configCrypto, jwtService, openid4vpDefSvc,
		runtimeStoreProvider)
	fatalOnError(ctx, logger, err, "Failed to initialize OpenID4VP verifier service")

	openid4vciCredSvc, vciCredExp, err := credential.Initialize(mux, ouService)
	fatalOnError(ctx, logger, err, "Failed to initialize credential configuration service")
	if vciCredExp != nil {
		exporters = append(exporters, vciCredExp)
	}

	_, err = openid4vci.Initialize(mux, runtimeCrypto, jwtService, userService, dpopVerifier, openid4vciCredSvc,
		runtimeStoreProvider)
	fatalOnError(ctx, logger, err, "Failed to initialize OpenID4VCI issuer service")

	return openid4vpSvc, openid4vpDefSvc, openid4vciCredSvc, exporters
}

// buildHashConfig constructs a cryptolib.HashConfig from the server configuration.
func buildHashConfig() (cryptolib.HashConfig, error) {
	cfg := config.GetServerRuntime().Config.Crypto.PasswordHashing
	alg := cryptolib.CredAlgorithm(strings.ToUpper(cfg.Algorithm))
	switch alg {
	case "", cryptolib.SHA256:
		return cryptolib.HashConfig{Algorithm: cryptolib.SHA256, SaltSize: cfg.SHA256.SaltSize}, nil
	case cryptolib.PBKDF2:
		return cryptolib.HashConfig{Algorithm: alg, SaltSize: cfg.PBKDF2.SaltSize,
			Iterations: cfg.PBKDF2.Iterations, KeySize: cfg.PBKDF2.KeySize}, nil
	case cryptolib.ARGON2ID:
		return cryptolib.HashConfig{Algorithm: alg, SaltSize: cfg.Argon2ID.SaltSize,
			Iterations: cfg.Argon2ID.Iterations, Memory: cfg.Argon2ID.Memory,
			Parallelism: cfg.Argon2ID.Parallelism, KeySize: cfg.Argon2ID.KeySize}, nil
	default:
		return cryptolib.HashConfig{}, fmt.Errorf("unrecognized password hashing algorithm %q", cfg.Algorithm)
	}
}
