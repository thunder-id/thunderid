// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package server holds the wiring both planes share.
//
// A plane is a binary: the data plane serves runtime traffic and the management APIs, the control
// plane serves the management APIs alone. What they have in common is built here once, so a service
// added to the product reaches both rather than whichever main was remembered.
package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/actorprovider"
	"github.com/thunder-id/thunderid/internal/agent"
	"github.com/thunder-id/thunderid/internal/agentmgtprovider"
	"github.com/thunder-id/thunderid/internal/application"
	"github.com/thunder-id/thunderid/internal/attributecache"
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
	"github.com/thunder-id/thunderid/internal/authnprovider/defaultprovider"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/authnprovider/restprovider"
	"github.com/thunder-id/thunderid/internal/authz"
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
	"github.com/thunder-id/thunderid/internal/flow/flowmeta"
	"github.com/thunder-id/thunderid/internal/flow/graphbuilder"
	"github.com/thunder-id/thunderid/internal/flow/interceptor"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	flowsession "github.com/thunder-id/thunderid/internal/flow/session"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	"github.com/thunder-id/thunderid/internal/notification"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/dpop"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jti"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/revocation"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/tokenservice"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/runtimestore"
	"github.com/thunder-id/thunderid/internal/serverconfig"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cors"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/csp"
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
	"github.com/thunder-id/thunderid/internal/usermgtprovider"
	"github.com/thunder-id/thunderid/internal/vc/credential"
	"github.com/thunder-id/thunderid/internal/vc/presentation"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// observabilitySvc is the observability service instance. This is used for graceful shutdown.
var observabilitySvc observability.ObservabilityServiceInterface

// BuildManagement wires everything both planes serve: the infrastructure a server needs and the
// management APIs it exposes. What it returns is the seam a plane's runtime wiring builds on.
func BuildManagement(mux *http.ServeMux, cacheManager cache.CacheManagerInterface) *Services {
	logger := log.GetLogger()

	// Service registration runs during application startup, outside any request.
	ctx := context.Background()

	// Load the server's private key for signing JWTs.
	pkiService, err := pki.Initialize()
	FatalOnError(ctx, logger, err, "Failed to initialize certificate service")

	runtimeCryptoSvc, configCryptoSvc, err := kmprovider.Initialize(pkiService)
	FatalOnError(ctx, logger, err, "Failed to initialize key manager provider")
	// Inject the ConfigCryptoProvider into cmodels. This breaks the import cycle that would
	// arise if cmodels were to directly import the kmprovider/defaultkm package.
	cmodels.SetConfigCryptoProvider(configCryptoSvc)

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
	FatalOnError(ctx, logger, err, "Failed to initialize JOSE services")

	observabilitySvc = observability.Initialize(config.GetServerRuntime().Config.Observability)

	// Initialize MCP server early so packages initializing below can register tools.
	// Route mounting (mcp.Initialize) happens later in main(), once the token-revocation enforcer
	// exists — mcp.DefaultGuard needs it to reject revoked tokens the same way the REST gate does.
	mcpServer := mcp.NewServer()

	// List to collect exporters from each package
	var exporters []declarativeresource.ResourceExporter

	// Initialize i18n service for internationalization support.
	i18nService, i18nExporter, err := i18nmgt.Initialize(mux, config.GetServerRuntime().Config.Translation)
	FatalOnError(ctx, logger, err, "Failed to initialize i18n service")
	// Add to exporters list (must be done after initializing list)
	exporters = append(exporters, i18nExporter)

	ouAuthzService, err := sysauthz.Initialize()
	FatalOnError(ctx, logger, err, "Failed to initialize system authorization service")

	ouService, ouHierarchyResolver, ouExporter, err := ou.Initialize(mux, mcpServer, cacheManager, ouAuthzService)
	FatalOnError(ctx, logger, err, "Failed to initialize OrganizationUnitService")
	exporters = append(exporters, ouExporter)

	// Complete the two-phase initialization: inject the OU hierarchy resolver into the
	// authz service now that the ou package is ready. This breaks the import-cycle that
	// would arise if sysauthz were to directly import the ou package.
	ouAuthzService.SetOUHierarchyResolver(ouHierarchyResolver)

	hashCfg, err := buildHashConfig()
	FatalOnError(ctx, logger, err, "Failed to build HashService config")
	hashService, err := cryptolib.Initialize(hashCfg)
	FatalOnError(ctx, logger, err, "Failed to initialize HashService")

	// Initialize user type service
	entityTypeService, entityTypeExporter, agentTypeExporter, err := entitytype.Initialize(
		mux, mcpServer, cacheManager, ouService, ouAuthzService)
	FatalOnError(ctx, logger, err, "Failed to initialize EntityTypeService")
	exporters = append(exporters, entityTypeExporter)
	exporters = append(exporters, agentTypeExporter)

	// Initialize entity service
	entityService, err := entity.Initialize(cacheManager, hashService, entityTypeService, ouService)
	FatalOnError(ctx, logger, err, "Failed to initialize EntityService")

	// Initialize entity provider
	entityProvider := entityprovider.InitializeEntityProvider(entityService)

	userService, ouUserResolver, userExporter, err := user.Initialize(
		mux, entityService, ouService, entityTypeService, ouAuthzService,
	)
	FatalOnError(ctx, logger, err, "Failed to initialize UserService")
	exporters = append(exporters, userExporter)

	// Initialize user management provider
	userMgtProvider := usermgtprovider.Initialize(userService)

	groupService, ouGroupResolver, groupExporter, err := group.Initialize(
		mux, dbprovider.GetDBProvider(), ouService, entityService, entityTypeService, ouAuthzService,
	)
	FatalOnError(ctx, logger, err, "Failed to initialize GroupService")
	exporters = append(exporters, groupExporter)

	resourceService, resourceExporter, err := resource.Initialize(mux, ouService)
	FatalOnError(ctx, logger, err, "Failed to initialize Resource Service")
	exporters = append(exporters, resourceExporter)

	roleService, roleAssignmentService, ouRoleResolver, roleExporter, err := role.Initialize(
		mux, entityService, groupService, ouService, resourceService, entityTypeService, ouAuthzService,
	)
	FatalOnError(ctx, logger, err, "Failed to initialize RoleService")
	exporters = append(exporters, roleExporter)

	// Two-phase initialization: inject user/group/role resolvers into OU service.
	ouService.SetOUUserResolver(ouUserResolver)
	ouService.SetOUGroupResolver(ouGroupResolver)
	ouService.SetOURoleResolver(ouRoleResolver)

	// Complete the two-phase initialization of the privilege-escalation guard. The resolver spans
	// roles, groups, and entities, so it can only be built once all three are ready. Until it is
	// injected the guard fails closed, so this must not be skipped.
	ouAuthzService.SetPermissionResolver(
		role.NewEffectivePermissionResolver(roleService, groupService, entityService))

	authZService := authz.Initialize(roleService)

	idpService, err := idp.Initialize(cacheManager, entityTypeService)
	FatalOnError(ctx, logger, err, "Failed to initialize IDPService")

	templateService, err := template.Initialize()
	FatalOnError(ctx, logger, err, "Failed to initialize template service")

	notifSenderMgtSvc, notifOTPService, notifSenderSvc, err := notification.Initialize(jwtService)
	FatalOnError(ctx, logger, err, "Failed to initialize NotificationService")

	// Register the /connections API as a thin layer over the identity-provider and
	// notification-sender services.
	connectionExporter, err := connection.Initialize(mux, idpService, notifSenderMgtSvc)
	FatalOnError(ctx, logger, err, "Failed to initialize connection declarative resources")
	exporters = append(exporters, connectionExporter)

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

	runtimeStoreProvider, transactioner, err := runtimestore.Initialize(runtime.Config.Database.RuntimeTransient.Type,
		runtime.Config.Server.Identifier)
	FatalOnError(ctx, logger, err, "Failed to initialize runtime store")

	// Initialize passkey service
	passkeyService := passkey.Initialize(entityService, runtimeStoreProvider)

	// Shared DPoP verifier (and its JTI replay cache) so OAuth and OpenID4VCI
	// share JTI replay protection.
	oauthCfg := oauthconfig.FromServerRuntime()
	dpopVerifier := dpop.Initialize(oauthCfg, jti.Initialize(runtimeStoreProvider), runtimeCryptoSvc)

	openid4vpSvc, openid4vpDefSvc, openid4vciCredSvc, exporters :=
		initializeVCServices(ctx, logger, mux, runtimeCryptoSvc, configCryptoSvc, jwtService,
			ouService, runtimeStoreProvider, exporters)

	defaultProvider := defaultprovider.Initialize(entityService, passkeyService,
		otpCoreService, magicLinkService, openid4vpSvc, federatedAuths)

	customProviders := map[string]providers.CustomAuthnProvider{}
	restCfg := runtime.Config.AuthnProvider.Rest
	if restCfg.Enabled {
		restProvider, err := restprovider.Initialize(restCfg)
		FatalOnError(ctx, logger, err, "Failed to initialize REST authn provider")
		customProviders[restprovider.Name] = providers.CustomAuthnProvider{
			Instance: restProvider,
			Creds:    restCfg.CredentialTypes,
		}
	}
	authnProvider, err := authnprovidermgr.Initialize(defaultProvider, customProviders)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize authn provider manager", log.Error(err))
	}

	// Initialize authentication services.
	authAssertGen := authnAssert.Initialize()
	consentEnforcer := authnConsent.Initialize(jwtService)

	attributeCacheService := attributecache.Initialize(runtimeStoreProvider, runtimeCryptoSvc,
		runtime.Config.AttributeCache.Encryption.Enabled)

	emailClient := initEmailClient(ctx, logger)

	// Create the flow server-config handler early so it can be registered before serverconfig is
	// initialized. The handle-existence validator is injected in a second phase after flowMgtService
	// is available.
	flowConfigHandler := flowmgt.NewFlowConfigHandler()

	// Initialize server-wide configuration after its handler dependencies.
	serverConfigHandlers := map[serverconfig.ConfigName]serverconfig.ServerConfigHandlerInterface{
		serverconfig.ConfigNameCORS:                  cors.OriginHandler{},
		serverconfig.ConfigNameDefaultResourceServer: resource.NewDefaultResourceServerConfigHandler(resourceService),
		serverconfig.ConfigNameSession:               flowsession.ConfigHandler{},
		serverconfig.ConfigNameFlow:                  flowConfigHandler,
		serverconfig.ConfigNameCSP:                   csp.PolicyHandler{},
	}
	serverConfigService, serverConfigExporter, err := serverconfig.Initialize(mux, cacheManager, serverConfigHandlers)
	FatalOnError(ctx, logger, err, "Failed to initialize server config service")
	exporters = append(exporters, serverConfigExporter)

	// CORS origins come from the server-config cors section.
	cors.InitializeDynamicMatcher(serverConfigService)

	// Decorate the resource provider so an empty identifier resolves the configured default resource
	// server. Keeps the default-resource-server policy server-side: OAuth, CIBA, PAR, grant handlers,
	// and the flow executor depend only on providers.ResourceServerProvider, not on serverConfigService.
	// The base resourceService is still used for resource-management APIs and server-config validation.
	resourceServerProvider := resource.NewDefaultAwareResourceServerProvider(resourceService, serverConfigService)

	// The security-headers middleware reads the deny-first CSP policy from the server-config csp section.
	csp.InitializeConfigReader(serverConfigService)

	flowConfig := flowconfig.FromServerRuntime()
	tokenFamilyRevocationTTL := time.Duration(runtime.Config.OAuth.RefreshToken.ValidityPeriod) * time.Second
	revocationEnforcer, revocationSvc := revocation.Initialize(jwtService, observabilitySvc,
		tokenFamilyRevocationTTL, runtime.Config.OAuth.Revocation.TokenFamily.OnExplicitRevokeEnabled())
	sessionRevoker := sessionCriteriaRevoker{revoker: revocationSvc}
	sessionService, sessionCfg := initSessionService(ctx, serverConfigService,
		runtime.Config.Server.Identifier, sessionRevoker, logger)
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
			MagicLinkService:      magicLinkService,
			AuthZService:          authZService,
			EntityTypeService:     entityTypeService,
			GroupService:          groupService,
			RoleService:           roleService,
			RoleAssignmentService: roleAssignmentService,
			EntityProvider:        entityProvider,
			UserMgtProvider:       userMgtProvider,
			AttributeCacheSvc:     attributeCacheService,
			EmailClient:           emailClient,
			TemplateService:       templateService,
			OAuthSvc:              oauthAuthnService,
			OIDCSvc:               oidcAuthnService,
			GithubSvc:             githubAuthnService,
			GoogleSvc:             googleAuthnService,
			OpenID4VPVerifierSvc:  openid4vpSvc,
			SessionService:        sessionService,
			ResourceService:       resourceServerProvider,
			UserService:           userService,
			CriteriaRevoker:       revocationSvc,
		},
		interceptor.InterceptorDependencies{},
		flowConfig,
	)

	flowMgtService, flowMgtExporter, err := flowmgt.Initialize(
		mux, mcpServer, cacheManager, flowFactory, execRegistry, interceptorRegistry, graphBuilder,
		serverConfigService, ouService, flowConfigHandler)
	FatalOnError(ctx, logger, err, "Failed to initialize FlowMgtService")

	// Two-phase initialization: inject the flow resolver into the OU service.
	ouService.SetOUFlowResolver(flowMgtService)

	exporters = append(exporters, flowMgtExporter)
	certservice, err := cert.Initialize(cacheManager, dbprovider.GetDBProvider())
	FatalOnError(ctx, logger, err, "Failed to initialize CertificateService")

	// Initialize theme and layout services
	themeMgtService, themeExporter, err := thememgt.Initialize(mux, mcpServer)
	FatalOnError(ctx, logger, err, "Failed to initialize ThemeMgtService")
	exporters = append(exporters, themeExporter)

	layoutMgtService, layoutExporter, err := layoutmgt.Initialize(mux)
	FatalOnError(ctx, logger, err, "Failed to initialize LayoutMgtService")
	exporters = append(exporters, layoutExporter)

	inboundClientService, err := inboundclient.Initialize(
		cacheManager, certservice, entityProvider,
		themeMgtService, layoutMgtService, flowMgtService, entityTypeService, runtimeCryptoSvc, jweService)
	FatalOnError(ctx, logger, err, "Failed to initialize InboundClientService")

	// Inject the consent service into the consent enforcer. It is wired here rather than at enforcer
	// construction because it depends on the inbound client service, which is only available after the
	// flow services (which themselves depend on the enforcer) are initialized.
	consentEnforcer.SetConsentService(initConsentService(ctx, logger, inboundClientService))

	applicationService, applicationExporter, err := application.Initialize(
		mux, mcpServer, entityService, inboundClientService, ouService, i18nService,
		runtimeCryptoSvc, serverConfigService,
		func(client *providers.OAuthClient) time.Duration {
			return tokenservice.ArtifactLifetime(oauthCfg, client)
		})
	FatalOnError(ctx, logger, err, "Failed to initialize ApplicationService")
	// Two-phase initialization: inject the application service into the executors that act on it.
	FatalOnError(ctx, logger, executor.SetApplicationProvider(execRegistry, applicationService),
		"Failed to inject the application provider into the flow executors")
	exporters = append(exporters, applicationExporter)

	agentService, agentExporter, err := agent.Initialize(mux, entityService, inboundClientService, ouService,
		roleService)
	FatalOnError(ctx, logger, err, "Failed to initialize AgentService")
	exporters = append(exporters, agentExporter)

	// Initialize agent management provider. It has no runtime consumer yet: the provisioning
	// executor gains its agent branch in a follow-up change, at which point this is handed to the
	// executor registry. It is constructed here so the package is linked into the server binary and
	// its integration coverage is reported as uncovered rather than silently dropped.
	// TODO: pass to the provisioning executor once agent provisioning lands.
	_ = agentmgtprovider.Initialize(agentService)

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
	}, applicationService, agentService, flowMgtService, roleAssignmentService, roleService,
		groupService, ouService, ouUserResolver, ouGroupResolver, resourceService)

	// Initialize design resolve service for theme and layout resolution
	designResolveService := resolve.Initialize(mux, themeMgtService, layoutMgtService, applicationService)

	actorProvider := actorprovider.Initialize(inboundClientService, entityProvider, authnProvider, roleService)

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

	// Register the health service.
	healthSvc := healthcheckservice.Initialize(dbprovider.GetDBProvider(), dbprovider.GetRedisProvider())
	services.NewHealthCheckService(mux, healthSvc)

	return &Services{
		Mux:                    mux,
		Logger:                 logger,
		MCPServer:              mcpServer,
		JWTService:             jwtService,
		JWEService:             jweService,
		RuntimeCryptoSvc:       runtimeCryptoSvc,
		ImportService:          importService,
		ObservabilitySvc:       observabilitySvc,
		OUService:              ouService,
		I18nService:            i18nService,
		IDPService:             idpService,
		UserService:            userService,
		ApplicationService:     applicationService,
		ResourceService:        resourceService,
		AuthZService:           authZService,
		EntityProvider:         entityProvider,
		ActorProvider:          actorProvider,
		AuthnProvider:          authnProvider,
		AuthAssertGen:          authAssertGen,
		FlowMgtService:         flowMgtService,
		ServerConfigService:    serverConfigService,
		SessionService:         sessionService,
		AttributeCacheService:  attributeCacheService,
		RuntimeStoreProvider:   runtimeStoreProvider,
		Transactioner:          transactioner,
		DPoPVerifier:           dpopVerifier,
		RevocationEnforcer:     revocationEnforcer,
		RevocationSvc:          revocationSvc,
		ResourceServerProvider: resourceServerProvider,
		ExecRegistry:           execRegistry,
		InterceptorRegistry:    interceptorRegistry,
		GraphBuilder:           graphBuilder,
		FlowConfig:             flowConfig,
		OAuthCfg:               oauthCfg,
		OpenID4VCICredSvc:      openid4vciCredSvc,
		OpenID4VPSvc:           openid4vpSvc,
		OTPCoreService:         otpCoreService,
		NotifSenderSvc:         notifSenderSvc,
		TemplateService:        templateService,
		MagicLinkService:       magicLinkService,
		OAuthAuthnService:      oauthAuthnService,
		OIDCAuthnService:       oidcAuthnService,
		GoogleAuthnService:     googleAuthnService,
		GitHubAuthnService:     githubAuthnService,
		DirectAuthSecret:       runtime.Config.Server.SecurityConfig.DirectAuthSecret,
	}
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

// Unregister tears down the services that need it at shutdown.
func Unregister() {
	// A plane that failed before the shared build, or a one-shot subcommand that never ran it, has
	// nothing to tear down. Shutting down is meant to be safe to call on any exit path.
	if observabilitySvc == nil {
		return
	}
	observabilitySvc.Shutdown()
}

// initSessionService reads the effective SSO session configuration from the server-config section and
// builds the session service, returning both so the caller can thread the config into flowexec too.
func initSessionService(ctx context.Context, svc serverconfig.ServerConfigService, deploymentID string,
	criteriaRevoker flowsession.CriteriaRevoker, logger *log.Logger) (flowsession.Service, flowsession.Config) {
	cfg := readSessionConfig(ctx, svc, logger)
	sessionService, err := flowsession.Initialize(dbprovider.GetDBProvider(), deploymentID,
		flowsession.NewTimeouts(cfg.IdleTimeoutSeconds, cfg.AbsoluteTimeoutSeconds,
			cfg.ActivityRefreshIntervalSeconds), criteriaRevoker)
	FatalOnError(ctx, logger, err, "Failed to initialize SSO session service")
	return sessionService, cfg
}

// sessionCriteriaRevoker fixes the reason used when the session service revokes a token family.
type sessionCriteriaRevoker struct {
	revoker revocation.CriteriaRevokerInterface
}

// RevokeTokenFamily revokes the given token family with the session-logout reason.
func (a sessionCriteriaRevoker) RevokeTokenFamily(ctx context.Context, tokenFamilyID string) error {
	return a.revoker.RevokeTokenFamily(ctx, tokenFamilyID, revocation.RevocationReasonSessionLogout)
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
	FatalOnError(ctx, logger, err, "Failed to initialize consent service")
	return consentService
}

// FatalOnError terminates startup when a service cannot be built. A plane that came up with a
// service missing would fail later, on a request, in a way that reads as a bug rather than as a
// server that never started.
func FatalOnError(ctx context.Context, logger *log.Logger, err error, msg string) {
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
	FatalOnError(ctx, logger, err, "Failed to register flow executors")
	interceptorRegistry, err := interceptor.Initialize(interceptorDeps, flowConfig.Flow)
	FatalOnError(ctx, logger, err, "Failed to initialize Interceptor registry")

	graphBuilder := graphbuilder.Initialize(flowFactory, execRegistry, interceptorRegistry, graphCache)

	return flowFactory, execRegistry, interceptorRegistry, graphBuilder
}

// initializeVCServices initializes the OpenID4VP verifier and the credential-configuration
// service, appending their declarative-resource exporters to exporters. The OpenID4VCI issuer
// itself is initialized later, once the application service it depends on exists.
func initializeVCServices(
	ctx context.Context, logger *log.Logger, mux *http.ServeMux,
	runtimeCrypto providers.RuntimeCryptoProvider, configCrypto kmprovider.ConfigCryptoProvider,
	jwtService jwt.JWTServiceInterface,
	ouService ou.OrganizationUnitServiceInterface,
	runtimeStoreProvider providers.RuntimeStoreProvider,
	exporters []declarativeresource.ResourceExporter,
) (openid4vp.Service, presentation.PresentationDefinitionServiceInterface,
	credential.CredentialConfigurationServiceInterface, []declarativeresource.ResourceExporter) {
	openid4vpDefSvc, vpDefExp, err := presentation.Initialize(mux, ouService)
	FatalOnError(ctx, logger, err, "Failed to initialize presentation definition service")
	if vpDefExp != nil {
		exporters = append(exporters, vpDefExp)
	}

	// The verifier is built here because the management surface reads presentation definitions
	// through it. Its wallet and verifier endpoints are runtime, so they are mounted by the plane
	// that serves them, not here.
	openid4vpSvc, err := openid4vp.Initialize(runtimeCrypto, configCrypto, jwtService, openid4vpDefSvc,
		runtimeStoreProvider)
	FatalOnError(ctx, logger, err, "Failed to initialize OpenID4VP verifier service")

	openid4vciCredSvc, vciCredExp, err := credential.Initialize(mux, ouService)
	FatalOnError(ctx, logger, err, "Failed to initialize credential configuration service")
	if vciCredExp != nil {
		exporters = append(exporters, vciCredExp)
	}

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
