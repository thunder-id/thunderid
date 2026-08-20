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
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package main is the Control Plane (CP) server entry point. It registers only the
// management services: resource CRUD APIs, their configuration/entity persistence, and
// management-time validation. Runtime (Data Plane) components are deliberately not
// wired here: no OAuth2/OIDC token issuance, no user login/authn, no flow execution,
// no authorization evaluation, and no verifiable-credential issuance/verification.
//
// This file is the CP counterpart of cmd/server/servicemanager.go. Anything present
// there but absent here is a Data Plane concern that the CP build drops.
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
	"github.com/thunder-id/thunderid/internal/dataplanetoken"
	layoutmgt "github.com/thunder-id/thunderid/internal/design/layout/mgt"
	thememgt "github.com/thunder-id/thunderid/internal/design/theme/mgt"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/environmentvariable"
	"github.com/thunder-id/thunderid/internal/envmgr"
	"github.com/thunder-id/thunderid/internal/envmgr/dataplane"
	"github.com/thunder-id/thunderid/internal/envmgr/jobrunner"
	"github.com/thunder-id/thunderid/internal/envmgr/secretcapture"
	envmgrservice "github.com/thunder-id/thunderid/internal/envmgr/service"
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
	"github.com/thunder-id/thunderid/internal/system/channel"
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

// channelServer is the CP-DP phone-home WebSocket server. It is held here so unregisterServices can
// close it during shutdown. Nil when the channel is disabled.
var channelServer *channel.Server

// dataPlaneTokens issues and holds the credential each data plane presents when it connects.
var dataPlaneTokens *dataplanetoken.Service

// registerServices registers the Control Plane management services with the provided HTTP
// multiplexer. It also returns the import and export services so the bootstrap and export
// subcommands can create or read resources in-process through the same service instances.
func registerServices(mux *http.ServeMux, cacheManager cache.CacheManagerInterface) (
	jwt.JWTServiceInterface, kmprovider.RuntimeCryptoProvider,
	importer.ImportServiceInterface, export.ExportServiceInterface, envmgrRegistry,
	environmentvariable.EnvironmentVariableServiceInterface) {
	logger := log.GetLogger()

	// Service registration runs during application startup, outside any request.
	ctx := context.Background()

	// Load the server's private key. On the CP this signs the management API surface's JWTs and
	// provides the config-at-rest encryption key used by the application service; it does not issue
	// end-user access/ID tokens (that is a Data Plane responsibility).
	pkiService, err := pki.Initialize()
	fatalOnError(ctx, logger, err, "Failed to initialize certificate service")

	runtimeCryptoSvc, configCryptoSvc, err := kmprovider.Initialize(pkiService)
	fatalOnError(ctx, logger, err, "Failed to initialize key manager provider")

	envVarService, err := environmentvariable.Initialize(mux)
	fatalOnError(ctx, logger, err, "Failed to initialize EnvironmentVariableService")

	// Promotion between deployments is a control plane concern, so the environment manager is wired
	// only here. It stays disabled unless a data directory is configured, which leaves a deployment
	// using the standalone service untouched.
	//
	// It is initialized before the capturer because it is where a captured credential goes when this
	// server hosts it.
	envManager := initEnvironmentManager(ctx, logger, mux, config.GetServerRuntime().Config)

	// A captured credential is handed to the Data Plane's secret service, so the Control Plane holds no
	// secret at rest and the credential is usable immediately rather than at the next promotion.
	secretCapturer := secretcapture.Select(ctx, logger, config.GetServerRuntime().Config, envManager,
		buildHashConfig)

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
		mux, entityService, ouService, entityTypeService, ouAuthzService, secretCapturer,
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

	idpService, err := idp.Initialize(cacheManager, entityTypeService)
	fatalOnError(ctx, logger, err, "Failed to initialize IDPService")

	// Notification: the CP keeps only the sender-management (CRUD) service. The OTP and
	// sender runtime services returned here are Data Plane concerns and are dropped.
	notifSenderMgtSvc, _, _, err := notification.Initialize(jwtService)
	fatalOnError(ctx, logger, err, "Failed to initialize NotificationService")

	// Register the /connections API as a thin layer over the identity-provider and
	// notification-sender management services.
	connectionExporter, err := connection.Initialize(mux, idpService, notifSenderMgtSvc, secretCapturer)
	fatalOnError(ctx, logger, err, "Failed to initialize connection declarative resources")
	exporters = append(exporters, connectionExporter)

	// Server-wide configuration handlers. The CP omits the "session" handler because SSO session
	// lifetime is a Data Plane (flow/session) concern.
	flowConfigHandler := flowmgt.NewFlowConfigHandler()

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

	// Verifiable-credential DEFINITION management. The CP stores presentation definitions and
	// credential configurations; the OpenID4VP verifier and OpenID4VCI issuer runtimes are dropped.
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

	// Flow MANAGEMENT (CRUD + definition validation).
	//
	flowConfig := flowconfig.FromServerRuntime()
	flowFactory, execRegistry, interceptorRegistry, graphBuilder := initializeFlowValidation(ctx, logger,
		cacheManager, interceptor.InterceptorDependencies{}, flowConfig)

	flowMgtService, flowMgtExporter, err := flowmgt.Initialize(
		mux, mcpServer, cacheManager, flowFactory, execRegistry, interceptorRegistry, graphBuilder,
		serverConfigService, ouService, flowConfigHandler)
	fatalOnError(ctx, logger, err, "Failed to initialize FlowMgtService")
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
		runtimeCryptoSvc, serverConfigService, secretCapturer)
	fatalOnError(ctx, logger, err, "Failed to initialize ApplicationService")
	exporters = append(exporters, applicationExporter)

	agentService, agentExporter, err := agent.Initialize(mux, entityService, inboundClientService, ouService,
		roleService, secretCapturer)
	fatalOnError(ctx, logger, err, "Failed to initialize AgentService")
	exporters = append(exporters, agentExporter)

	// Wire the dependency registry into the consuming services (two-phase init to avoid cyclic
	// imports). All consumers and providers here are management services.
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

	// Initialize export service with collected exporters
	exportService := export.Initialize(mux, exporters)

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

	// A capture reads the organization's workspace, which is this very server, so the environment
	// manager is told where that answers.
	if envManager != nil {
		envManager.SetWorkspaceURL(envmgr.WorkspaceURL(config.GetServerRuntime().Config))
	}

	// Register the health service.
	healthSvc := healthcheckservice.Initialize(dbprovider.GetDBProvider(), dbprovider.GetRedisProvider())
	services.NewHealthCheckService(mux, healthSvc)

	// Initialize the CP-DP channel server (phone-home WebSocket). No-op when disabled.
	chCfg := config.GetServerRuntime().Config.Channel.Server
	// Tokens are issued per data plane when an environment is registered and held encrypted here, so
	// the handshake knows which data plane connected instead of believing the id it claims.
	dataPlaneTokens = dataplanetoken.New()
	// A credential queued for a data plane is encrypted at rest with the server's configuration key.
	if envManager != nil && configCryptoSvc != nil {
		envManager.SetSecretSealer(jobrunner.NewConfigSealer(configCryptoSvc))
	}

	channelServer = channel.InitializeServer(mux, channel.ServerConfig{
		Enabled:    chCfg.Enabled,
		Path:       chCfg.Path,
		AuthToken:  chCfg.AuthToken,
		ReadLimit:  chCfg.ReadLimitBytes,
		RPCTimeout: time.Duration(chCfg.RPCTimeoutSeconds) * time.Second,
	}, dataPlaneTokens)

	// Data planes are reached over the connections they hold open to this server, so the environment
	// manager is given those rather than a client for each one's management API. Without a channel
	// there is no way to reach a data plane at all, and an apply says so rather than failing obscurely.
	if envManager != nil {
		envManager.SetDataPlanes(dataplane.New(channelServer))
		envManager.SetDataPlaneTokenIssuer(dataPlaneTokens)
		// Work queued by a pod that held no connection to its data plane waits in the database. This
		// is what picks up the share of it this pod can deliver.
		jobrunner.Start(ctx, envManager, channelServer)
	}

	return jwtService, runtimeCryptoSvc, importService, exportService, envManager, envVarService
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
	if channelServer != nil {
		channelServer.Close()
	}
}

// fatalOnError logs msg and exits the process if err is non-nil.
func fatalOnError(ctx context.Context, logger *log.Logger, err error, msg string) {
	if err != nil {
		logger.Fatal(ctx, msg, log.Error(err))
	}
}

// initializeFlowCoreAndExecutor initializes the flow core and executor registries used for flow
// definition validation. On the CP the executor dependencies carry only management services; the
// runtime dependencies are left nil because validation reads static executor metadata only.
// initializeFlowValidation initializes what flow definition validation and graph building need.
//
// The executor side is the static metadata catalog rather than the real registry: this plane
// validates flows and never runs one, and constructing the executors would link the data-plane
// services they are built with. The catalog honors the same configured subset, so a flow accepted
// here is one the plane that runs it has registered.
func initializeFlowValidation(
	ctx context.Context,
	logger *log.Logger,
	cacheManager cache.CacheManagerInterface,
	interceptorDeps interceptor.InterceptorDependencies,
	flowConfig flowconfig.Config,
) (flowcore.FlowFactoryInterface, flowcore.ExecutorMetadataProvider,
	interceptor.InterceptorRegistryInterface, graphbuilder.GraphBuilderInterface) {
	flowFactory, graphCache := flowcore.Initialize(cacheManager)
	interceptorDeps.FlowFactory = flowFactory

	execRegistry, err := executormeta.NewRegistry(flowConfig.Flow.Executors)
	fatalOnError(ctx, logger, err, "Failed to build the flow executor metadata registry")
	interceptorRegistry, err := interceptor.Initialize(interceptorDeps, flowConfig.Flow)
	fatalOnError(ctx, logger, err, "Failed to initialize Interceptor registry")

	graphBuilder := graphbuilder.Initialize(flowFactory, execRegistry, interceptorRegistry, graphCache)

	return flowFactory, execRegistry, interceptorRegistry, graphBuilder
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

// initEnvironmentManager mounts the in-process environment manager. A failure is logged rather than
// fatal: promotion is one capability of the control plane, and losing it should not stop the
// management APIs from serving.
// It returns the manager when one is mounted, so captured credentials can be routed through it instead
// of over HTTP; a nil result means promotion is not hosted here.
func initEnvironmentManager(ctx context.Context, logger *log.Logger, mux *http.ServeMux,
	cfg config.Config) envmgrRegistry {
	// The environment manager keeps its environments and versions in their own datasource. Leaving it
	// unconfigured is how a deployment that promotes by other means turns the module off, and without
	// it there is nowhere for a captured version to go.
	if strings.TrimSpace(cfg.Database.Environment.Type) == "" {
		logger.Info(ctx, "No environment datasource is configured, so promotion is not hosted here")
		return nil
	}
	reg, err := envmgr.Initialize(mux, secretcapture.HasherFor(buildHashConfig))
	if err != nil {
		logger.Error(ctx, "Failed to start the in-process environment manager", log.Error(err))
		return nil
	}
	logger.Info(ctx, "Serving the environment manager from this server")
	return reg
}

// envmgrRegistry is what the in-process environment manager exposes to this server: where a captured
// credential goes, and which control plane a promotion writes into.
type envmgrRegistry interface {
	secretcapture.LocalCaptureRouter
	SetWorkspaceURL(baseURL string)
	SetDataPlanes(planes envmgrservice.DataPlanes)
	SetDataPlaneTokenIssuer(issuer envmgrservice.DataPlaneTokenIssuer)
	SetSecretSealer(sealer envmgrservice.SecretSealer)
	// DeliverPending carries out work queued for a data plane this pod holds a connection to.
	DeliverPending(ctx context.Context, dataPlaneID string) error
	CreateEnvironment(ctx context.Context, deploymentID string,
		in envmgrservice.CreateEnvironmentInput) (envmgrservice.CreateEnvironmentResult, error)
}
