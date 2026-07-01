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

// Package thunderidengine provides the core engine for the Thunder ID platform.
package thunderidengine

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/internal/attributecache"
	"github.com/thunder-id/thunderid/internal/authn/assert"
	"github.com/thunder-id/thunderid/internal/authz"
	flowconfig "github.com/thunder-id/thunderid/internal/flow/config"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/executor"
	"github.com/thunder-id/thunderid/internal/flow/flowexec"
	"github.com/thunder-id/thunderid/internal/flow/flowmeta"
	"github.com/thunder-id/thunderid/internal/flow/graphbuilder"
	"github.com/thunder-id/thunderid/internal/flow/interceptor"
	"github.com/thunder-id/thunderid/internal/oauth"
	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/jose"
	joseconfig "github.com/thunder-id/thunderid/internal/system/jose/config"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/jose/jwt"
	"github.com/thunder-id/thunderid/internal/system/kmprovider"
	"github.com/thunder-id/thunderid/internal/system/kmprovider/defaultkm/pki"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// Engine is the ThunderID runtime engine that wires core platform services.
type Engine struct {
	engineCtx *engineContext
}

// New creates and initializes a ThunderID engine with the given HTTP mux and options.
func New(mux *http.ServeMux, opts ...Option) *Engine {
	logger := log.GetLogger()
	ctx := context.Background()

	var engineCtx engineContext
	for _, opt := range opts {
		opt(&engineCtx)
	}

	// Initialize the cache manager.
	engineCtx.cacheManager = cache.Initialize(engineCtx.cacheConfig, engineCtx.serverConfig.Identifier)
	// Load the server's private key for signing JWTs.
	pkiService, err := pki.Initialize()
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize certificate service", log.Error(err))
	}
	engineCtx.runtimeCryptoSvc, _, err = kmprovider.Initialize(pkiService)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize key manager provider", log.Error(err))
	}
	joseCfg := joseconfig.Config{
		Issuer:         engineCtx.jwtConfig.Issuer,
		ValidityPeriod: engineCtx.jwtConfig.ValidityPeriod,
		Audience:       engineCtx.jwtConfig.Audience,
		PreferredKeyID: engineCtx.jwtConfig.PreferredKeyID,
		Leeway:         engineCtx.jwtConfig.Leeway,
		JWKSCacheTTL:   time.Duration(engineCtx.serverConfig.SecurityConfig.JWKSCacheTTL) * time.Second,
	}
	engineCtx.jwtService, engineCtx.jweService, err = jose.Initialize(engineCtx.runtimeCryptoSvc, joseCfg)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize JOSE services", log.Error(err))
	}

	engineCtx.observabilitySvc = observability.Initialize(engineCtx.observabilityConfig)
	attributeCacheService := attributecache.Initialize()
	authZService := authz.Initialize(engineCtx.roleProvider)
	engineCtx.authAssertGen = assert.Initialize()

	// Initialize flow metadata service
	_ = flowmeta.Initialize(mux, engineCtx.actorProvider, engineCtx.ouProvider,
		engineCtx.designResolveProvider, engineCtx.i18nProvider)

	// Initialize flow core services.
	flowConfig := flowconfig.Config{
		Flow:          engineCtx.flowConfig,
		DeploymentID:  engineCtx.serverConfig.Identifier,
		RuntimeDBType: engineCtx.runtimeDBType,
	}
	flowFactory, graphCache := core.Initialize(engineCtx.cacheManager)
	engineCtx.flowFactory = flowFactory
	execDeps := executor.ExecutorDependencies{
		FlowFactory:       engineCtx.flowFactory,
		AttributeCacheSvc: attributeCacheService,
		AuthZService:      authZService,
		ConsentEnforcer:   engineCtx.consentProvider,
		AuthnProvider:     engineCtx.authnProvider,
		JWTService:        engineCtx.jwtService,
		AuthAssertGen:     engineCtx.authAssertGen,
	}
	interceptorDeps := interceptor.InterceptorDependencies{
		FlowFactory: engineCtx.flowFactory,
	}

	engineCtx.execRegistry, err = executor.Initialize(execDeps, flowConfig.Flow)
	if err != nil {
		logger.Fatal(ctx, "Failed to register flow executors", log.Error(err))
	}
	err = engineCtx.applyCustomExecutors()
	if err != nil {
		logger.Fatal(ctx, "Failed to apply custom executors", log.Error(err))
	}

	engineCtx.interceptorRegistry, err = interceptor.Initialize(interceptorDeps, flowConfig.Flow)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize Interceptor registry", log.Error(err))
	}

	engineCtx.graphBuilder = graphbuilder.Initialize(engineCtx.flowFactory, engineCtx.execRegistry,
		engineCtx.interceptorRegistry, graphCache)

	flowExecService, err := flowexec.Initialize(mux, engineCtx.flowProvider, engineCtx.actorProvider,
		engineCtx.execRegistry, engineCtx.interceptorRegistry, engineCtx.observabilitySvc,
		engineCtx.runtimeCryptoSvc, engineCtx.graphBuilder, flowConfig)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize flow execution service", log.Error(err))
	}

	oauthConfig := oauthconfig.Config{
		DeploymentID:  engineCtx.serverConfig.Identifier,
		RuntimeDBType: engineCtx.runtimeDBType,
		BaseURL:       config.GetServerURL(&engineCtx.serverConfig),
		JWT:           engineCtx.jwtConfig,
		OAuth:         engineCtx.oauthConfig,
		GateClient:    engineCtx.gateClientConfig,
	}
	err = oauth.Initialize(mux, engineCtx.actorProvider, engineCtx.authnProvider, engineCtx.jwtService,
		engineCtx.jweService, flowExecService, engineCtx.observabilitySvc, engineCtx.runtimeCryptoSvc,
		engineCtx.ouProvider, attributeCacheService, authZService, engineCtx.resourceProvider,
		engineCtx.i18nProvider, engineCtx.idpProvider, nil, oauthConfig)
	if err != nil {
		logger.Fatal(ctx, "Failed to initialize OAuth services", log.Error(err))
	}

	return &Engine{
		engineCtx: &engineCtx,
	}
}

// applyCustomExecutors registers the custom executors with the executor registry.
func (e *engineContext) applyCustomExecutors() error {
	if len(e.customExecutors) == 0 {
		return nil
	}
	if e.execRegistry == nil {
		return errors.New("thunderidengine: executor registry is nil")
	}
	for name, ex := range e.customExecutors {
		e.execRegistry.RegisterExecutor(name, ex)
	}
	return nil
}

type engineContext struct {
	cacheManager        cache.CacheManagerInterface
	jwtService          jwt.JWTServiceInterface
	jweService          jwe.JWEServiceInterface
	runtimeCryptoSvc    kmprovider.RuntimeCryptoProvider
	observabilitySvc    observability.ObservabilityServiceInterface
	flowFactory         core.FlowFactoryInterface
	execRegistry        executor.ExecutorRegistryInterface
	interceptorRegistry interceptor.InterceptorRegistryInterface
	graphBuilder        graphbuilder.GraphBuilderInterface
	authAssertGen       assert.AuthAssertGeneratorInterface

	serverHome          string
	runtimeDBType       string
	oauthConfig         engineconfig.OAuthConfig
	jwtConfig           engineconfig.JWTConfig
	flowConfig          engineconfig.FlowConfig
	serverConfig        engineconfig.ServerConfig
	cacheConfig         engineconfig.CacheConfig
	observabilityConfig engineconfig.ObservabilityConfig
	gateClientConfig    engineconfig.GateClientConfig

	actorProvider         providers.ActorProvider
	authnProvider         providers.AuthnProviderManager
	resourceProvider      providers.ResourceServerProvider
	ouProvider            providers.OrganizationUnitProvider
	designResolveProvider providers.DesignProvider
	flowProvider          providers.FlowProvider
	i18nProvider          providers.I18nProvider
	roleProvider          providers.RoleProvider
	idpProvider           providers.IDPProvider
	consentProvider       providers.ConsentProvider
	customExecutors       map[string]providers.Executor
}

// Option configures engine initialization.
type Option func(*engineContext)

// WithServerHome supplies the server home directory used for all runtime
// state. Required.
func WithServerHome(serverHome string) Option {
	return func(c *engineContext) { c.serverHome = serverHome }
}

// WithServerConfig supplies the server configuration.
func WithServerConfig(config engineconfig.ServerConfig) Option {
	return func(c *engineContext) { c.serverConfig = config }
}

// WithCacheConfig supplies the cache configuration.
func WithCacheConfig(config engineconfig.CacheConfig) Option {
	return func(c *engineContext) { c.cacheConfig = config }
}

// WithOAuthConfig supplies the OAuth configuration.
func WithOAuthConfig(config engineconfig.OAuthConfig) Option {
	return func(c *engineContext) { c.oauthConfig = config }
}

// WithJWTConfig supplies the JWT configuration.
func WithJWTConfig(config engineconfig.JWTConfig) Option {
	return func(c *engineContext) { c.jwtConfig = config }
}

// WithFlowConfig supplies the flow configuration.
func WithFlowConfig(config engineconfig.FlowConfig) Option {
	return func(c *engineContext) { c.flowConfig = config }
}

// WithObservabilityConfig supplies the observability configuration.
func WithObservabilityConfig(config engineconfig.ObservabilityConfig) Option {
	return func(c *engineContext) { c.observabilityConfig = config }
}

// WithActorProvider supplies the actor provider.
func WithActorProvider(provider providers.ActorProvider) Option {
	return func(c *engineContext) { c.actorProvider = provider }
}

// WithAuthnProvider supplies the authentication provider manager.
func WithAuthnProvider(provider providers.AuthnProviderManager) Option {
	return func(c *engineContext) { c.authnProvider = provider }
}

// WithResourceProvider supplies the resource provider.
func WithResourceProvider(provider providers.ResourceServerProvider) Option {
	return func(c *engineContext) { c.resourceProvider = provider }
}

// WithOUProvider supplies the organization unit provider.
func WithOUProvider(provider providers.OrganizationUnitProvider) Option {
	return func(c *engineContext) { c.ouProvider = provider }
}

// WithDesignResolveProvider supplies the design resolve provider.
func WithDesignResolveProvider(provider providers.DesignProvider) Option {
	return func(c *engineContext) { c.designResolveProvider = provider }
}

// WithFlowProvider supplies the flow provider.
func WithFlowProvider(provider providers.FlowProvider) Option {
	return func(c *engineContext) { c.flowProvider = provider }
}

// WithI18nProvider supplies the i18n provider.
func WithI18nProvider(provider providers.I18nProvider) Option {
	return func(c *engineContext) { c.i18nProvider = provider }
}

// WithConsentProvider supplies the consent provider.
func WithConsentProvider(provider providers.ConsentProvider) Option {
	return func(c *engineContext) { c.consentProvider = provider }
}

// WithCustomExecutors supplies the custom executors to be registered with the engine.
func WithCustomExecutors(executors map[string]providers.Executor) Option {
	return func(c *engineContext) {
		if c.customExecutors == nil {
			c.customExecutors = make(map[string]providers.Executor, len(executors))
		}
		for name, ex := range executors {
			c.customExecutors[name] = ex
		}
	}
}
