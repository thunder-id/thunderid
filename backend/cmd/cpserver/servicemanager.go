// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package main is the Control Plane (CP) server entry point. It registers only the management
// services: the resource CRUD APIs, the persistence behind them, and management-time validation.
//
// Runtime components are deliberately absent: no OAuth2/OIDC token issuance, no login or
// registration, no flow execution, no authorization evaluation, and no verifiable-credential
// issuance or verification. Anything present in cmd/server/servicemanager.go but absent here is a
// data plane concern that this build drops.
//
// Flow definitions are still validated, from the static executor catalog in flow/executormeta
// rather than from a live executor registry, so the executors and the services they need are never
// constructed or linked.
package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/agent"
	"github.com/thunder-id/thunderid/internal/application"
	"github.com/thunder-id/thunderid/internal/cert"
	"github.com/thunder-id/thunderid/internal/connection"
	layoutmgt "github.com/thunder-id/thunderid/internal/design/layout/mgt"
	thememgt "github.com/thunder-id/thunderid/internal/design/theme/mgt"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
	flowcore "github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/executormeta"
	"github.com/thunder-id/thunderid/internal/flow/graphbuilder"
	"github.com/thunder-id/thunderid/internal/flow/interceptor"
	flowmgt "github.com/thunder-id/thunderid/internal/flow/mgt"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	"github.com/thunder-id/thunderid/internal/notification"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/serverconfig"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cors"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/csp"
	dbprovider "github.com/thunder-id/thunderid/internal/system/database/provider"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
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
	"github.com/thunder-id/thunderid/internal/user"
	"github.com/thunder-id/thunderid/internal/vc/credential"
	"github.com/thunder-id/thunderid/internal/vc/presentation"
)

// observabilitySvc is the observability service instance. This is used for graceful shutdown.
var observabilitySvc observability.ObservabilityServiceInterface

// registerServices registers the Control Plane management services with the provided HTTP
// multiplexer. It also returns the import service so the bootstrap subcommand can create default
// resources in-process through the same service instances.
// nolint:gocyclo // This is the main service registration function, so its length is expected to be proportional
// to the number of services. Eventhough it has many branching statements, almost all are early exits so cognitive
// complexity is low.
func registerServices(mux *http.ServeMux, cacheManager cache.CacheManagerInterface) (
	jwt.JWTServiceInterface, kmprovider.RuntimeCryptoProvider, importer.ImportServiceInterface) {
	logger := log.GetLogger()

	// Service registration runs during application startup, outside any request.
	ctx := context.Background()

	// Load the server's private key. Here this signs the management API surface's JWTs and provides
	// the config-at-rest encryption key; it issues no end-user access or ID tokens, which are a data
	// plane responsibility.
	pkiService, err := pki.Initialize()
	fatalOnError(ctx, logger, err, "Failed to initialize certificate service")

	runtimeCryptoSvc, configCryptoSvc, err := kmprovider.Initialize(pkiService)
	fatalOnError(ctx, logger, err, "Failed to initialize key manager provider")
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
	fatalOnError(ctx, logger, err, "Failed to initialize JOSE services")

	observabilitySvc = observability.Initialize(config.GetServerRuntime().Config.Observability)

	// Initialize MCP server early so packages initializing below can register tools.
	mcpServer := mcp.Initialize(mux, jwtService)

	// List to collect exporters from each package
	var exporters []declarativeresource.ResourceExporter

	// Initialize i18n service for internationalization support.
	i18nService, i18nExporter, err := i18nmgt.Initialize(mux, config.GetServerRuntime().Config.Translation)
	fatalOnError(ctx, logger, err, "Failed to initialize i18n service")
	exporters = append(exporters, i18nExporter)

	ouAuthzService, err := sysauthz.Initialize()
	fatalOnError(ctx, logger, err, "Failed to initialize system authorization service")

	ouService, ouHierarchyResolver, ouExporter, err := ou.Initialize(mux, mcpServer, cacheManager, ouAuthzService)
	fatalOnError(ctx, logger, err, "Failed to initialize OrganizationUnitService")
	exporters = append(exporters, ouExporter)

	// Complete the two-phase initialization: inject the OU hierarchy resolver into the
	// authz service now that the ou package is ready.
	ouAuthzService.SetOUHierarchyResolver(ouHierarchyResolver)

	hashCfg, err := buildHashConfig()
	fatalOnError(ctx, logger, err, "Failed to build HashService config")
	hashService, err := cryptolib.Initialize(hashCfg)
	fatalOnError(ctx, logger, err, "Failed to initialize HashService")

	entityTypeService, entityTypeExporter, agentTypeExporter, err := entitytype.Initialize(
		mux, mcpServer, cacheManager, ouService, ouAuthzService)
	fatalOnError(ctx, logger, err, "Failed to initialize EntityTypeService")
	exporters = append(exporters, entityTypeExporter)
	exporters = append(exporters, agentTypeExporter)

	entityService, err := entity.Initialize(cacheManager, hashService, entityTypeService, ouService)
	fatalOnError(ctx, logger, err, "Failed to initialize EntityService")

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
		mux, entityService, groupService, ouService, resourceService, entityTypeService, ouAuthzService,
	)
	fatalOnError(ctx, logger, err, "Failed to initialize RoleService")
	exporters = append(exporters, roleExporter)

	// Two-phase initialization: inject user/group/role resolvers into OU service.
	ouService.SetOUUserResolver(ouUserResolver)
	ouService.SetOUGroupResolver(ouGroupResolver)
	ouService.SetOURoleResolver(ouRoleResolver)

	// Complete the two-phase initialization of the privilege-escalation guard. The resolver spans
	// roles, groups, and entities, so it can only be built once all three are ready. Until it is
	// injected the guard fails closed, so this must not be skipped: granting a permission is
	// authoring, which this plane does, and without the resolver every such grant is refused.
	ouAuthzService.SetPermissionResolver(
		role.NewEffectivePermissionResolver(roleService, groupService, entityService))

	idpService, err := idp.Initialize(cacheManager, entityTypeService)
	fatalOnError(ctx, logger, err, "Failed to initialize IDPService")

	// Notification: only the sender-management (CRUD) service is kept. The OTP and sender runtime
	// services this returns are data plane concerns and are dropped.
	notifSenderMgtSvc, _, _, err := notification.Initialize(jwtService)
	fatalOnError(ctx, logger, err, "Failed to initialize NotificationService")

	// Register the /connections API as a thin layer over the identity-provider and
	// notification-sender management services.
	connectionExporter, err := connection.Initialize(mux, idpService, notifSenderMgtSvc)
	fatalOnError(ctx, logger, err, "Failed to initialize connection declarative resources")
	exporters = append(exporters, connectionExporter)

	// Create the flow server-config handler early so it can be registered before serverconfig is
	// initialized. The handle-existence validator is injected in a second phase after flowMgtService
	// is available.
	flowConfigHandler := flowmgt.NewFlowConfigHandler()

	// Server-wide configuration handlers. The "session" handler is omitted because SSO session
	// lifetime is a data plane concern.
	serverConfigHandlers := map[serverconfig.ConfigName]serverconfig.ServerConfigHandlerInterface{
		serverconfig.ConfigNameCORS:                  cors.OriginHandler{},
		serverconfig.ConfigNameDefaultResourceServer: resource.NewDefaultResourceServerConfigHandler(resourceService),
		serverconfig.ConfigNameFlow:                  flowConfigHandler,
		serverconfig.ConfigNameCSP:                   csp.PolicyHandler{},
	}
	serverConfigService, serverConfigExporter, err := serverconfig.Initialize(mux, cacheManager, serverConfigHandlers)
	fatalOnError(ctx, logger, err, "Failed to initialize server config service")
	exporters = append(exporters, serverConfigExporter)

	// CORS origins come from the server-config cors section.
	cors.InitializeDynamicMatcher(serverConfigService)

	// The security-headers middleware reads the deny-first CSP policy from the server-config csp section.
	csp.InitializeConfigReader(serverConfigService)

	// Verifiable-credential DEFINITION management. Presentation definitions and credential
	// configurations are authored here; the OpenID4VP verifier and OpenID4VCI issuer runtimes are
	// data plane concerns and are dropped.
	openid4vpDefSvc, vpDefExp, err := presentation.Initialize(mux, ouService)
	fatalOnError(ctx, logger, err, "Failed to initialize presentation definition service")
	if vpDefExp != nil {
		exporters = append(exporters, vpDefExp)
	}

	openid4vciCredSvc, vciCredExp, err := credential.Initialize(mux, ouService)
	fatalOnError(ctx, logger, err, "Failed to initialize credential configuration service")
	if vciCredExp != nil {
		exporters = append(exporters, vciCredExp)
	}

	// Flow MANAGEMENT: CRUD plus definition validation, with no execution behind it.
	flowFactory, execRegistry, interceptorRegistry, graphBuilder := initializeFlowValidation(ctx, logger,
		cacheManager, flowconfig.FromServerRuntime())

	flowMgtService, flowMgtExporter, err := flowmgt.Initialize(
		mux, mcpServer, cacheManager, flowFactory, execRegistry, interceptorRegistry, graphBuilder,
		serverConfigService, ouService, flowConfigHandler)
	fatalOnError(ctx, logger, err, "Failed to initialize FlowMgtService")

	// Two-phase initialization: inject the flow resolver into the OU service.
	ouService.SetOUFlowResolver(flowMgtService)
	exporters = append(exporters, flowMgtExporter)

	certservice, err := cert.Initialize(cacheManager, dbprovider.GetDBProvider())
	fatalOnError(ctx, logger, err, "Failed to initialize CertificateService")

	themeMgtService, themeExporter, err := thememgt.Initialize(mux, mcpServer)
	fatalOnError(ctx, logger, err, "Failed to initialize ThemeMgtService")
	exporters = append(exporters, themeExporter)

	layoutMgtService, layoutExporter, err := layoutmgt.Initialize(mux)
	fatalOnError(ctx, logger, err, "Failed to initialize LayoutMgtService")
	exporters = append(exporters, layoutExporter)

	inboundClientService, err := inboundclient.Initialize(
		cacheManager, certservice, entityProvider,
		themeMgtService, layoutMgtService, flowMgtService, entityTypeService, runtimeCryptoSvc, jweService)
	fatalOnError(ctx, logger, err, "Failed to initialize InboundClientService")

	// TODO: Remove entityService dependency after finalizing declarative resource loading pattern
	applicationService, applicationExporter, err := application.Initialize(
		mux, mcpServer, entityProvider, entityService, inboundClientService, ouService, i18nService,
		runtimeCryptoSvc, serverConfigService)
	fatalOnError(ctx, logger, err, "Failed to initialize ApplicationService")
	exporters = append(exporters, applicationExporter)

	agentService, agentExporter, err := agent.Initialize(mux, entityService, inboundClientService, ouService,
		roleService)
	fatalOnError(ctx, logger, err, "Failed to initialize AgentService")
	exporters = append(exporters, agentExporter)

	// Wire the dependency registry into the consuming services (two-phase init to avoid cyclic
	// imports). flowMgtService is both a consumer and a provider: it reports which flows reference an
	// identity provider or notification sender. Every consumer and provider here is a management
	// service.
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

	return jwtService, runtimeCryptoSvc, importService
}

// initializeFlowValidation builds the flow services needed to validate a flow definition, without
// an execution engine behind them. The executor registry is the static catalog from
// flow/executormeta rather than the live registry, so no executor is constructed and the runtime
// services executors depend on are never linked into this binary.
func initializeFlowValidation(
	ctx context.Context,
	logger *log.Logger,
	cacheManager cache.CacheManagerInterface,
	flowConfig flowconfig.Config,
) (flowcore.FlowFactoryInterface, flowcore.ExecutorMetadataProvider,
	interceptor.InterceptorRegistryInterface, graphbuilder.GraphBuilderInterface) {
	flowFactory, graphCache := flowcore.Initialize(cacheManager)

	execRegistry, err := executormeta.NewRegistry(flowConfig.Flow.Executors)
	fatalOnError(ctx, logger, err, "Failed to build the executor metadata registry")

	interceptorRegistry, err := interceptor.Initialize(interceptor.InterceptorDependencies{
		FlowFactory: flowFactory,
	}, flowConfig.Flow)
	fatalOnError(ctx, logger, err, "Failed to initialize Interceptor registry")

	graphBuilder := graphbuilder.Initialize(flowFactory, execRegistry, interceptorRegistry, graphCache)

	return flowFactory, execRegistry, interceptorRegistry, graphBuilder
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

// fatalOnError logs msg and exits the process if err is non-nil.
func fatalOnError(ctx context.Context, logger *log.Logger, err error, msg string) {
	if err != nil {
		logger.Fatal(ctx, msg, log.Error(err))
	}
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
