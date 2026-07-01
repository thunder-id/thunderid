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
	"github.com/thunder-id/thunderid/internal/authn/otp"
	"github.com/thunder-id/thunderid/internal/authn/passkey"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/authz"
	"github.com/thunder-id/thunderid/internal/authzen"
	"github.com/thunder-id/thunderid/internal/cert"
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
	openid4vcicred "github.com/thunder-id/thunderid/internal/openid4vci/credential"
	"github.com/thunder-id/thunderid/internal/openid4vp"
	openid4vpdef "github.com/thunder-id/thunderid/internal/openid4vp/definition"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/role"
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
	"github.com/thunder-id/thunderid/internal/system/services"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/internal/system/template"
	"github.com/thunder-id/thunderid/internal/user"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// observabilitySvc is the observability service instance. This is used for graceful shutdown.
var observabilitySvc observability.ObservabilityServiceInterface

// registerServices registers all the services with the provided HTTP multiplexer.
func registerServices(mux *http.ServeMux, cacheManager cache.CacheManagerInterface) (
	jwt.JWTServiceInterface, kmprovider.RuntimeCryptoProvider) {
	logger := log.GetLogger()

	// Service registration runs during application startup, outside any request.
	ctx := context.Background()

	// Load the server's private key for signing JWTs.
	pkiService, err := pki.Initialize()
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize certificate service", log.Error(err))
	}

	runtimeCryptoSvc, configCryptoSvc, err := kmprovider.Initialize(pkiService)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize key manager provider", log.Error(err))
	}

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
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize JOSE services", log.Error(err))
	}

	observabilitySvc = observability.Initialize(config.GetServerRuntime().Config.Observability)

	// Initialize MCP server early so packages initializing below can register tools.
	mcpServer := mcp.Initialize(mux, jwtService)

	// List to collect exporters from each package
	var exporters []declarativeresource.ResourceExporter

	// Initialize i18n service for internationalization support.
	i18nService, i18nExporter, err := i18nmgt.Initialize(mux, config.GetServerRuntime().Config.Translation)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize i18n service", log.Error(err))
	}
	// Add to exporters list (must be done after initializing list)
	exporters = append(exporters, i18nExporter)

	// Initialize the server-wide configuration service with the CORS section handler.
	serverConfigHandlers := map[serverconfig.ConfigName]serverconfig.ServerConfigHandlerInterface{
		serverconfig.ConfigNameCORS: cors.OriginHandler{},
	}
	serverConfigService, serverConfigExporter, err := serverconfig.Initialize(mux, cacheManager, serverConfigHandlers)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize server config service", log.Error(err))
	}
	exporters = append(exporters, serverConfigExporter)

	ouAuthzService, err := sysauthz.Initialize()
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize system authorization service", log.Error(err))
	}

	ouService, ouHierarchyResolver, ouExporter, err := ou.Initialize(mux, mcpServer, cacheManager, ouAuthzService)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize OrganizationUnitService", log.Error(err))
	}
	exporters = append(exporters, ouExporter)

	// Complete the two-phase initialization: inject the OU hierarchy resolver into the
	// authz service now that the ou package is ready. This breaks the import-cycle that
	// would arise if sysauthz were to directly import the ou package.
	ouAuthzService.SetOUHierarchyResolver(ouHierarchyResolver)

	hashCfg, err := buildHashConfig()
	if err != nil {
		logger.Fatal(ctx, "Failed to build HashService config", log.Error(err))
	}
	hashService, err := cryptolib.Initialize(hashCfg)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize HashService", log.Error(err))
	}

	// Initialize consent service
	consentService := consent.Initialize()

	// Initialize user type service
	entityTypeService, entityTypeExporter, err := entitytype.Initialize(
		mux, mcpServer, cacheManager, ouService, ouAuthzService, consentService)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize EntityTypeService", log.Error(err))
	}
	exporters = append(exporters, entityTypeExporter)

	// Initialize entity service
	entityService, err := entity.Initialize(cacheManager, hashService, entityTypeService, ouService)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize EntityService", log.Error(err))
	}

	// Initialize entity provider
	entityProvider := entityprovider.InitializeEntityProvider(entityService)

	userService, ouUserResolver, userExporter, err := user.Initialize(
		mux, entityService, ouService, entityTypeService, ouAuthzService,
	)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize UserService", log.Error(err))
	}
	exporters = append(exporters, userExporter)

	groupService, ouGroupResolver, groupExporter, err := group.Initialize(
		mux, dbprovider.GetDBProvider(), ouService, entityService, entityTypeService, ouAuthzService,
	)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize GroupService", log.Error(err))
	}
	exporters = append(exporters, groupExporter)

	// Two-phase initialization: inject user/group resolvers into OU service.
	ouService.SetOUUserResolver(ouUserResolver)
	ouService.SetOUGroupResolver(ouGroupResolver)

	resourceService, resourceExporter, err := resource.Initialize(mux, ouService, consentService)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize Resource Service", log.Error(err))
	}
	exporters = append(exporters, resourceExporter)
	roleService, roleAssignmentService, roleExporter, err := role.Initialize(
		mux, entityService, groupService, ouService, resourceService, entityTypeService,
	)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize RoleService", log.Error(err))
	}
	exporters = append(exporters, roleExporter)
	authZService := authz.Initialize(roleService)
	authzen.Initialize(mux, authZService, entityProvider, resourceService)

	idpService, idpExporter, err := idp.Initialize(cacheManager, mux, entityTypeService)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize IDPService", log.Error(err))
	}
	exporters = append(exporters, idpExporter)

	templateService, err := template.Initialize()
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize template service", log.Error(err))
	}

	_, otpService, notifSenderSvc, notificationExporter, err := notification.Initialize(
		mux, jwtService, templateService)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize NotificationService", log.Error(err))
	}
	exporters = append(exporters, notificationExporter)

	// Initialize passkey service
	passkeyService := passkey.Initialize(entityService)

	// Initialize magic link service
	magicLinkService := magiclink.Initialize(jwtService)

	// Initialize otp core service
	otpCoreService := otp.Initialize(otpService)

	// Initialize federated authentication services.
	oauthAuthnService := authnOAuth.Initialize(idpService, entityProvider)
	oidcAuthnService := authnOIDC.Initialize(oauthAuthnService, jwtService, idpService)
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

	// Initialize the verifiable-credential services (OpenID4VP verifier + OpenID4VCI issuer).
	openid4vpVerifierSvc, openid4vpDefSvc, _, openid4vciCredSvc, exporters :=
		initializeVCServices(
			ctx, logger, mux, runtimeCryptoSvc, configCryptoSvc, jwtService, userService, ouService,
			dpopVerifier, exporters)

	// Initialize authn provider
	authnProvider := authnprovidermgr.InitializeAuthnProviderManager(entityService, passkeyService, otpCoreService,
		magicLinkService, openid4vpVerifierSvc, federatedAuths)

	// Initialize authentication services.
	authAssertGen := authnAssert.Initialize()
	consentEnforcer := authnConsent.Initialize(consentService, jwtService)

	authn.Initialize(mux, mcpServer, idpService, jwtService, authnProvider, authAssertGen, passkeyService,
		otpCoreService, magicLinkService, oauthAuthnService, oidcAuthnService, googleAuthnService, githubAuthnService)

	attributeCacheService := attributecache.Initialize()

	emailClient := initEmailClient(ctx, logger)
	flowConfig := flowconfig.FromServerRuntime()
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
			OpenID4VPVerifierSvc:  openid4vpVerifierSvc,
		},
		interceptor.InterceptorDependencies{},
		flowConfig,
	)

	flowMgtService, flowMgtExporter, err := flowmgt.Initialize(
		mux, mcpServer, cacheManager, flowFactory, execRegistry, interceptorRegistry, graphBuilder)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize FlowMgtService", log.Error(err))
	}

	exporters = append(exporters, flowMgtExporter)
	certservice, err := cert.Initialize(cacheManager, dbprovider.GetDBProvider())
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize CertificateService", log.Error(err))
	}

	// Initialize theme and layout services
	themeMgtService, themeExporter, err := thememgt.Initialize(mux, mcpServer)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize ThemeMgtService", log.Error(err))
	}
	exporters = append(exporters, themeExporter)

	layoutMgtService, layoutExporter, err := layoutmgt.Initialize(mux)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize LayoutMgtService", log.Error(err))
	}
	exporters = append(exporters, layoutExporter)

	inboundClientService, err := inboundclient.Initialize(
		cacheManager, certservice, entityProvider,
		themeMgtService, layoutMgtService, flowMgtService, entityTypeService, consentService)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize InboundClientService", log.Error(err))
	}

	// TODO: Remove entityService dependency after finalizing declarative resource loading pattern
	applicationService, applicationExporter, err := application.Initialize(
		mux, mcpServer, entityProvider, entityService, inboundClientService, ouService, i18nService)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize ApplicationService", log.Error(err))
	}
	exporters = append(exporters, applicationExporter)

	agentService, agentExporter, err := agent.Initialize(mux, entityService, inboundClientService, ouService)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize AgentService", log.Error(err))
	}
	exporters = append(exporters, agentExporter)

	// Initialize design resolve service for theme and layout resolution
	designResolveService := resolve.Initialize(mux, themeMgtService, layoutMgtService, applicationService)

	actorProvider := actorprovider.Initialize(inboundClientService, entityProvider)

	// Initialize flow metadata service
	_ = flowmeta.Initialize(mux, actorProvider, ouService, designResolveService, i18nService)

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

	flowExecService, err := flowexec.Initialize(mux, flowMgtService, actorProvider,
		execRegistry, interceptorRegistry, observabilitySvc, runtimeCryptoSvc, graphBuilder,
		flowconfig.FromServerRuntime())
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize flow execution service", log.Error(err))
	}

	// Initialize OAuth services.
	err = oauth.Initialize(mux, actorProvider, authnProvider, jwtService, jweService,
		flowExecService, observabilitySvc, runtimeCryptoSvc, ouService, attributeCacheService, authZService,
		resourceService, i18nService, idpService, dpopVerifier, oauthCfg)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize OAuth services", log.Error(err))
	}

	// Register OAuth2 DCR service.
	err = dcr.Initialize(mux, applicationService, ouService, i18nService, oauthCfg)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize OAuth2 DCR service", log.Error(err))
	}

	// Register the health service.
	healthSvc := healthcheckservice.Initialize(dbprovider.GetDBProvider(), dbprovider.GetRedisProvider())
	services.NewHealthCheckService(mux, healthSvc)

	return jwtService, runtimeCryptoSvc
}

// unregisterServices unregisters all services that require cleanup during shutdown.
func unregisterServices() {
	observabilitySvc.Shutdown()
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
	if err != nil {
		logger.Fatal(ctx, "Failed to register flow executors", log.Error(err))
	}
	interceptorRegistry, err := interceptor.Initialize(interceptorDeps, flowConfig.Flow)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize Interceptor registry", log.Error(err))
	}

	graphBuilder := graphbuilder.Initialize(flowFactory, execRegistry, interceptorRegistry, graphCache)

	return flowFactory, execRegistry, interceptorRegistry, graphBuilder
}

// initializeVCServices initializes the OpenID4VP verifier and OpenID4VCI issuer
// services, appending their declarative-resource exporters to exporters.
func initializeVCServices(
	ctx context.Context, logger *log.Logger, mux *http.ServeMux,
	runtimeCrypto kmprovider.RuntimeCryptoProvider, configCrypto kmprovider.ConfigCryptoProvider,
	jwtService jwt.JWTServiceInterface, userService user.UserServiceInterface,
	ouService ou.OrganizationUnitServiceInterface,
	dpopVer dpop.VerifierInterface,
	exporters []declarativeresource.ResourceExporter,
) (openid4vp.OpenID4VPServiceInterface, openid4vpdef.PresentationDefinitionServiceInterface,
	openid4vci.OpenID4VCIServiceInterface,
	openid4vcicred.CredentialConfigurationServiceInterface, []declarativeresource.ResourceExporter) {
	vpVerifier, vpDefSvc, vpDefExp, err := openid4vp.Initialize(mux, runtimeCrypto, configCrypto, jwtService, ouService)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize OpenID4VP verifier service", log.Error(err))
	}
	if vpDefExp != nil {
		exporters = append(exporters, vpDefExp)
	}

	vciSvc, vciCredSvc, vciExp, err := openid4vci.Initialize(
		mux, runtimeCrypto, jwtService, userService, dpopVer, ouService)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize OpenID4VCI issuer service", log.Error(err))
	}
	if vciExp != nil {
		exporters = append(exporters, vciExp)
	}

	return vpVerifier, vpDefSvc, vciSvc, vciCredSvc, exporters
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
